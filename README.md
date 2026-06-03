# goproxy

A high-performance reverse proxy and API gateway written in Go.

This project is being built as a learning-first infrastructure portfolio project. The goal is not just to ship features, but to understand the networking, concurrency, reliability, observability, Kubernetes, Terraform, and benchmarking concepts behind a production traffic layer.

## Current Milestone

**Phase 1: HTTP reverse proxy**

- [x] Initialize this folder as its own project repository
- [x] Archive the original project sketch
- [x] Add a single-upstream reverse proxy handler
- [x] Add a CLI entrypoint
- [x] Add integration tests with `httptest`
- [ ] Review Phase 1 concepts before moving to dynamic routing

## Learning Roadmap

Before each feature, study only the concepts needed for that milestone, then apply them directly in the project.

1. **HTTP reverse proxy**
   - `net/http`
   - `httputil.ReverseProxy`
   - request and response headers
   - streaming response bodies
   - upstream failures
   - `httptest`

2. **Dynamic routing**
   - host, method, and path-prefix matching
   - route precedence
   - handler composition

3. **Load balancing**
   - round-robin
   - least-connections
   - weighted balancing
   - concurrency-safe shared state

4. **Reliability**
   - health checks
   - rate limiting
   - circuit breakers
   - timeouts and cancellation

5. **Observability**
   - Prometheus counters
   - latency histograms
   - error rates
   - upstream health metrics

6. **Deployment and benchmarking**
   - Docker
   - Kubernetes with `kind`
   - Terraform Kubernetes provider
   - Nginx comparison benchmarks

## Architecture

The intended final request path is:

```text
Metrics wrapper
  -> Client identification and rate limit
  -> Router
  -> Circuit-breaker filtering
  -> Load balancer
  -> Reverse proxy
```

Phase 1 implements only the final reverse-proxy step against one upstream.

## Running Locally

Start a backend service, then run:

```bash
go run . -upstream http://localhost:8081 -listen :8080
```

Then send traffic through the proxy:

```bash
curl http://localhost:8080/
```

## Testing

```bash
go test ./...
go test -race ./...
```

## Benchmark Results

Pending measurement.

Performance claims, including the proposed `50k req/sec` target, should only be added to resume wording after reproducible benchmark results exist.

## Original Plan

The original submitted project sketch is archived at [docs/original-project-sketch.md](docs/original-project-sketch.md).
