# goproxy

`goproxy` is a reverse proxy and API gateway written in Go. The project is being built incrementally to explore the design and implementation of production traffic infrastructure.

The current implementation focuses on request forwarding, dynamic routing, upstream selection, and reliability controls. The CLI currently runs the single-upstream proxy; the routed runtime is available as a package and will be connected to configuration in a later phase.

## Current Status

Completed:

- HTTP reverse proxy with request and response forwarding
- Upstream validation and `502 Bad Gateway` handling
- Dynamic routing by host, HTTP method, and path prefix
- Longest-prefix route selection
- Round-robin load balancing
- Least-connections load balancing
- Weighted round-robin load balancing
- Active upstream health checks with configurable thresholds
- Per-client token-bucket rate limiting
- Per-upstream circuit breakers with closed, open, and half-open states
- Context cancellation for background health checks
- Unit, integration, concurrency, race-detector, and `go vet` coverage

Planned:

- YAML configuration and startup validation
- Configuration reload on `SIGHUP`
- Prometheus metrics
- Kubernetes deployment manifests
- Terraform-managed deployment
- Reproducible benchmark comparison with Nginx

## Architecture

The routed proxy processes requests through these stages:

```text
Incoming request
       |
       v
Route matching
       |
       v
Per-route rate limiting
       |
       v
Health and circuit-breaker filtering
       |
       v
Load balancer
       |
       v
Reverse proxy
       |
       v
Upstream service
```

Each stage is implemented as a small, testable component. Route configuration supports either a single upstream URL or a pool of weighted upstreams.

## Project Layout

```text
.
├── main.go
├── internal/
│   └── proxy/
│       ├── proxy.go
│       ├── router.go
│       ├── upstream.go
│       ├── balancer.go
│       ├── round_robin.go
│       ├── least_connections.go
│       ├── weighted_round_robin.go
│       ├── health.go
│       ├── rate_limiter.go
│       └── circuit_breaker.go
├── tests/
│   ├── proxy_test.go
│   ├── router_test.go
│   ├── health_test.go
│   └── *_test.go
└── docs/
    └── original-project-sketch.md
```

Tests next to implementation exercise package-level behavior and concurrency-sensitive internals. Tests in `tests/` exercise the exported proxy API through HTTP integration scenarios.

## Running Locally

Start an HTTP service on another port, then run the proxy:

```bash
go run . -upstream http://localhost:8081 -listen :8080
```

Send a request through the proxy:

```bash
curl http://localhost:8080/
```

The current CLI accepts one upstream. Multi-route configuration will be added with the YAML configuration phase.

## Testing

Run all tests:

```bash
go test ./...
```

Run the race detector:

```bash
go test -race ./...
```

Run static analysis:

```bash
go vet ./...
```

## Learning Goals

This project is used to practice:

- Go's `net/http` server model and concurrency patterns
- `httputil.ReverseProxy`
- Routing and longest-prefix matching
- Load-balancing algorithms and active-connection tracking
- Health-check design and context cancellation
- Token-bucket rate limiting
- Circuit-breaker state machines
- Integration testing with `httptest`
- Race detection and defensive API design
- Kubernetes, Terraform, observability, and benchmarking

## Roadmap

The work is organized into incremental phases:

1. Reverse proxy
2. Dynamic routing
3. Load balancing
4. Reliability: health checks, rate limiting, and circuit breakers
5. YAML configuration and observability
6. Kubernetes and Terraform deployment
7. Benchmarking and performance analysis

The private `plan.md` file contains the detailed implementation checklist and quality requirements used during development.

## Benchmarking

Benchmark results have not been published yet. The planned benchmark will compare throughput and latency percentiles against Nginx using documented hardware, software versions, workload, and commands.

## Original Project Sketch

The original project proposal is preserved in [docs/original-project-sketch.md](docs/original-project-sketch.md).
