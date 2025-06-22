package route

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aswinkm-tc/go-reverse-proxy/pkg/backend"
	"gopkg.in/yaml.v3"
)

type Route interface {
	Match(req *http.Request) (*Rule, bool)
	GetName() string
}

type HTTPRoute struct {
	Name      string   `json:"name"`
	Hostnames []string `json:"hostnames"`
	Rules     []Rule   `json:"rules"`
}

func (r *HTTPRoute) GetName() string {
	return r.Name
}

func (r *HTTPRoute) Match(req *http.Request) (*Rule, bool) {
	if !slices.Contains(r.Hostnames, req.Host) {
		return nil, false
	}

	for _, rule := range r.Rules {
		for _, match := range rule.Matches {
			if !strings.HasPrefix(req.URL.Path, match.Path) {
				continue
			}

			headersMatch := true
			for k, v := range match.Headers {
				if req.Header.Get(k) != v {
					headersMatch = false
					break
				}
			}

			if headersMatch {
				return &rule, true
			}
		}
	}

	return nil, false
}

type Rule struct {
	Name        string             `json:"name" yaml:"name"` // Name of the rule
	Matches     []*HTTPMatch       `json:"matches" yaml:"matches"`
	BackendRefs []*backend.Backend `json:"backendRefs" yaml:"backendRefs"`
	Timeout     *HttpTimeout       `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type HTTPMatch struct {
	Path    string            `json:"path,omitempty"`    // Path to match
	Headers map[string]string `json:"headers,omitempty"` // Headers to match
}

type HttpTimeout struct {
	RequestTimeout        *time.Duration `json:"requestTimeout,omitempty"`
	BackendRequestTimeout *time.Duration `json:"backendRequestTimeout,omitempty"`
}

func (t *HttpTimeout) UnmarshalYAML(value *yaml.Node) error {
	var raw map[string]string
	if err := value.Decode(&raw); err != nil {
		return err
	}

	if v, ok := raw["requestTimeout"]; ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid requestTimeout: %w", err)
		}
		t.RequestTimeout = &d
	}

	if v, ok := raw["backendRequestTimeout"]; ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid backendRequestTimeout: %w", err)
		}
		t.BackendRequestTimeout = &d
	}

	return nil
}
