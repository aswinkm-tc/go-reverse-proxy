package main

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/aswinkm-tc/go-reverse-proxy/pkg/route"
	"gopkg.in/yaml.v3"

	"github.com/aswinkm-tc/go-reverse-proxy/pkg/server"
)

func main() {
	log := slog.Default()
	conf := `name: route1
hostnames:
  - "localhost"
rules:
- name: rule1
  matches:
    - path: "/hello"
  backendRefs:
    - name: app1
      address: "http://localhost:8081"
    - name: app2
      address: "http://localhost:8082"
  timeout:
    backendRequestTimeout: 5s
- rule: rule2
  matches:
    - path: "/api"
  backendRefs:
    - name: app1
      address: "http://localhost:8081"
    - name: app2
      address: "http://localhost:8082"
    - name: app3
      address: "http://localhost:8083"
  timeout:
    backendRequestTimeout: 5s
`
	var httpRoute route.HTTPRoute

	if err := yaml.NewDecoder(strings.NewReader(conf)).Decode(&httpRoute); err != nil {
		log.Error("Failed to decode route configuration", "error", err)
		return
	}

	s := server.New(&httpRoute, log)
	go startBackend("localhost:8081")
	go startBackend("localhost:8082")
	go startBackend("localhost:8083")

	h := http.HandlerFunc(s.Handle)

	if err := http.ListenAndServe(":8080", h); err != nil {
		log.Error("Failed to start server", "error", err)
		return
	}
}

func startBackend(address string) {
	h := http.NewServeMux()

	h.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Request received at /", "server", address)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("You have reached the backend!"))
	})

	h.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Request received at /hello", "server", address)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from backend!"))
	})

	if err := http.ListenAndServe(address, h); err != nil {
		panic(err)
	}
}
