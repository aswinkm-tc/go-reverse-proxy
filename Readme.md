# go-reverse-proxy

A simple reverse proxy built in Golang that supports routing, load balancing, and custom backend configurations.

## Features
* **Reverse Proxy**: Forward incoming HTTP requests to backend services.
* **Routing**: Match requests based on hostname, path, and headers.
* **Load Balancing**: Round-robin load balancing across multiple backends.
* **Timeouts**: Configurable request and backend timeouts.
* **Dynamic Configuration**: Define routes and backends using YAML configuration.

## Getting Started

### Prerequisites
- Go 1.20 or later
- A basic understanding of HTTP and reverse proxies

### Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/aswinkm-tc/go-reverse-proxy.git
   cd go-reverse-proxy
   ```
2. Build the project:
   ```bash
   go build -o go-reverse-proxy
   ```
3. Run the reverse proxy:
   ```bash
   ./go-reverse-proxy
   ```

## How It Works
* The proxy reads the YAML configuration to define routes and backends.
* Incoming requests are matched against the defined routes based on hostname, path, and headers.
* Requests are forwarded to the appropriate backend using round-robin load balancing.
* Responses from the backend are returned to the client.