package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aswinkm-tc/go-reverse-proxy/pkg/route"
)

type Server struct {
	log             *slog.Logger
	mu              *sync.Mutex
	matchers        map[string][]route.Route
	roundRobinIndex map[string]map[string]int
}

func (s *Server) selectBackend(req *http.Request) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hostname := req.Host
	s.log.Info("Hostname for request", "hostname", hostname)
	if hostMatcher, ok := s.matchers[hostname]; ok {
		for _, match := range hostMatcher {
			if rule, ok := match.Match(req); ok {
				var idx int
				if len(rule.BackendRefs) > 0 {
					if _, ok := s.roundRobinIndex[hostname]; !ok {
						s.roundRobinIndex[hostname] = make(map[string]int)
					}
					if idx, ok = s.roundRobinIndex[hostname][match.GetName()]; ok {
						idx = (idx + 1) % len(rule.BackendRefs)
						s.roundRobinIndex[hostname][match.GetName()] = idx
					} else {
						s.roundRobinIndex[hostname][match.GetName()] = 0
					}

					return rule.BackendRefs[idx].Address.String(), nil
				}
			}
		}
	} else {
		return "", errors.New("no matchers found")
	}
	return "", errors.New("no backend found for hostname: " + hostname)
}

func (s *Server) AddMatcher(hostname string, m route.Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchers[hostname] = append(s.matchers[hostname], m)
	s.log.Info("Registered matcher", "hostname", hostname, "matcher", m)
}

func New(r *route.HTTPRoute, log *slog.Logger) *Server {
	s := &Server{
		mu:              new(sync.Mutex),
		matchers:        make(map[string][]route.Route),
		roundRobinIndex: make(map[string]map[string]int),
		log:             log,
	}
	for _, hostnames := range r.Hostnames {
		s.AddMatcher(hostnames, r)
	}

	return s
}

func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	backend, err := s.selectBackend(r)
	if err != nil {
		http.Error(w, "No backend available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	clone, err := cloneAndRewriteRequest(r, backend)
	if err != nil {
		http.Error(w, "Failed to clone request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	clone = clone.WithContext(ctx)

	// Forward the request to the backend
	resp, err := http.DefaultClient.Do(clone)
	if err != nil {
		http.Error(w, "Failed to forward request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		http.Error(w, "Failed to copy response body: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func cloneAndRewriteRequest(req *http.Request, targetURL string) (*http.Request, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	var body io.ReadCloser
	if req.Body != nil {
		// This assumes the body is repeatable (e.g. in memory or rewindable)
		// In production, you may need to buffer it
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		defer req.Body.Close()
		req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}

	// Clone the request to avoid modifying the original
	new := &http.Request{
		Body:          body,
		Method:        req.Method,
		URL:           url.ResolveReference(req.URL),
		ContentLength: req.ContentLength,
		Host:          url.Host,
		RemoteAddr:    req.RemoteAddr,
		RequestURI:    "",
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		TLS:           req.TLS,
		Header:        make(http.Header),
	}

	maps.Copy(new.Header, req.Header)

	// Add X-Forwarded-For header
	localPs, err := getLocalIPs()
	if err != nil {
		return nil, err
	}
	remoteIp, err := netip.ParseAddrPort(req.RemoteAddr)
	if err != nil {
		return nil, err
	}
	new.Header["X-Forwarded-For"] = append(new.Header["X-Forwared-For"], remoteIp.Addr().String())
	new.Header["X-forwarded-For"] = append(new.Header["X-forwarded-For"], localPs...)
	return new, nil
}

// getLocalIPs retrieves the local IP addresses of the machine, excluding loopback and non-IPv4 addresses
// This is used to populate the X-Forwarded-For header in the cloned request
func getLocalIPs() ([]string, error) {
	var ips []string

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipNet.IP

		// Skip loopback and non-ipv4 addresses
		if ip.IsLoopback() || ip.To4() == nil {
			continue
		}

		ips = append(ips, ip.String())
	}

	return ips, nil
}
