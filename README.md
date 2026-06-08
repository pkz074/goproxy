# goproxy

`goproxy` is a reverse proxy and API gateway written in Go.

The goal of this project is to build a traffic layer from the ground up and learn how production proxies handle routing, load balancing, rate limiting, reliability, observability, and deployment.

This is a work in progress. I am building it feature by feature instead of starting with a large framework or finished template.

## Current Status

Implemented:

- Single-upstream HTTP reverse proxy
- Request and response forwarding
- Upstream failure handling with `502 Bad Gateway`
- Dynamic route matching by path prefix, host, and HTTP method
- Longest-prefix route selection
- Routed proxy handler
- Integration tests using `httptest`

In progress:

- Expanding dynamic routing into a full runtime configuration model

Planned:

- Round-robin load balancing
- Least-connections load balancing
- Weighted load balancing
- Active upstream health checks
- Per-client rate limiting
- Circuit breaker support
- Prometheus metrics
- YAML config with reload support
- Kubernetes deployment
- Terraform-managed local deployment
- Benchmark comparison with Nginx

## Architecture

The final proxy pipeline is planned to look like this:

```text
Incoming request
  -> Metrics
  -> Rate limiter
  -> Router
  -> Circuit breaker
  -> Load balancer
  -> Reverse proxy
  -> Upstream service
```

The current implementation includes the router and reverse proxy pieces. The remaining middleware will be added incrementally.

## Project Layout

```text
.
├── main.go
├── internal/
│   └── proxy/
│       ├── proxy.go
│       └── router.go
├── tests/
│   ├── proxy_test.go
│   └── router_test.go
└── docs/
    └── original-project-sketch.md
```

## Running Locally

Start any HTTP backend on another port, then run:

```bash
go run . -upstream http://localhost:8081 -listen :8080
```

Send a request through the proxy:

```bash
curl http://localhost:8080/
```

For now, the CLI runs the single-upstream proxy. Routed configuration will be wired into the CLI in a later phase.

## Testing

Run the test suite:

```bash
go test ./...
```

Run the race detector:

```bash
go test -race ./...
```

## Learning Goals

The main topics I am using this project to practice are:

- Go's `net/http` server model
- `httputil.ReverseProxy`
- HTTP request routing
- concurrency-safe load balancing
- failure handling and retries
- health checks
- rate limiting algorithms
- circuit breaker state machines
- Prometheus metrics
- Kubernetes and Terraform deployment
- benchmark design and performance analysis

## Benchmarking

No benchmark numbers are published yet.

I plan to compare `goproxy` with Nginx after the core routing, load balancing, and reliability features are implemented. Any performance claim in this repository will be backed by reproducible commands and environment details.

## Original Plan

The initial project sketch is saved in [docs/original-project-sketch.md](docs/original-project-sketch.md).
