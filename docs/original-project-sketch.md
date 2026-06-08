# goproxy Project Sketch

This was the initial project plan for `goproxy`.

The idea is to build a reverse proxy and API gateway in Go, similar in purpose to the traffic layer handled by tools like Nginx, Caddy, and Traefik. The project is meant to cover routing, load balancing, reliability, observability, deployment, and benchmarking.

## Goals

- Build a reverse proxy from scratch in Go.
- Learn how traffic is routed between clients and backend services.
- Add production-style API gateway features one at a time.
- Deploy the proxy to Kubernetes.
- Use Terraform for local infrastructure setup.
- Benchmark the proxy against Nginx once the core features are complete.

## Planned Stack

- Go
- Kubernetes
- Terraform
- Prometheus
- Nginx for benchmark comparison

## Feature Order

The features should be built in order so each one is independently testable before the next one is added.

| Feature | Description |
|---|---|
| HTTP reverse proxy | Forward requests to upstream servers and stream responses back |
| Dynamic routing | Route by path prefix, host header, or HTTP method |
| Round-robin load balancing | Distribute requests evenly across an upstream pool |
| Least-connections load balancing | Route to the upstream with the fewest active requests |
| Weighted load balancing | Send different percentages of traffic to different upstreams |
| Health checks | Periodically check upstream health and avoid unhealthy targets |
| Rate limiting | Use a token bucket per client IP |
| Circuit breaker | Stop routing to failing upstreams and support half-open recovery |
| Prometheus metrics | Track request count, latency, upstream health, and error rate |
| Config file | Load routes and upstreams from YAML |
| Hot reload | Reload config on `SIGHUP` without restarting the proxy |
| Kubernetes deploy | Run the proxy in a local Kubernetes cluster |
| Terraform deploy | Manage local Kubernetes resources with Terraform |
| Nginx benchmark | Compare throughput and latency against Nginx |

## Planned Architecture

```text
Incoming request
        |
        v
Listener
        |
        v
Middleware chain
        |
        |-- Rate limiter
        |-- Router
        |-- Circuit breaker
        |-- Load balancer
        |-- Reverse proxy
        |-- Metrics recorder
        |
        v
Upstream pool
```

## Notes

The benchmark target is intentionally not listed as a result yet. Throughput and latency numbers should only be added after they are measured with reproducible commands.
