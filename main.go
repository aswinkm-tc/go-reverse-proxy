package main

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type Forwarder struct {
	backendURL  string
	backendPath string
}

func main() {

	f := &Forwarder{
		backendURL:  "http://localhost:8081",
		backendPath: "/backend",
	}

	go startBackend(f)

	h := http.HandlerFunc(f.Forward)

	if err := http.ListenAndServe(":8080", h); err != nil {
		panic(err)
	}
}

func (f *Forwarder) Forward(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	clone, err := cloneAndRewriteRequest(r, f.backendURL+f.backendPath)
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

func startBackend(f *Forwarder) {
	backendUrl, err := url.Parse(f.backendURL)
	if err != nil {
		panic(err)
	}

	h := http.NewServeMux()

	h.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Headers", "X-Forwarded-For", r.Header["X-Forwarded-For"])
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("You have reached the backend!"))
	})

	h.HandleFunc(f.backendPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from backend!"))
	})

	if err := http.ListenAndServe(backendUrl.Host, h); err != nil {
		panic(err)
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
	slog.Info("Local IPs", "ips", localPs)
	remoteIp, err := netip.ParseAddrPort(req.RemoteAddr)
	if err != nil {
		return nil, err
	}
	new.Header["X-Forwarded-For"] = append(new.Header["X-Forwared-For"], remoteIp.Addr().String())
	new.Header["X-forwarded-For"] = append(new.Header["X-forwarded-For"], localPs...)
	return new, nil
}

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
