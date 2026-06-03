# goproxy — Project Sketch

**Stack:** Go · Kubernetes · Terraform  
**Project:** `goproxy`  
**Description:** A high-performance reverse proxy and API gateway written in Go.

---

## 1. Overview

`goproxy` is a reverse proxy and API gateway built from scratch in Go.

It sits in front of backend services and handles:

- Routing
- Load balancing
- Rate limiting
- Circuit breaking
- Observability

This is the same layer that tools like **Nginx**, **Caddy**, and **Traefik** occupy in production systems.

### Resume One-Liner

> Built a reverse proxy and API gateway in Go with dynamic routing, load balancing using round-robin and least-connections, per-IP rate limiting, circuit breaking, and Prometheus metrics; deployed on Kubernetes with Terraform and benchmarked against Nginx at 50k req/sec.

### Why This Project Fits the Portfolio

- Completes the infrastructure story:
  - `Golem` = distributed scheduler
  - `mini-redis` = storage engine
  - `goproxy` = traffic layer
- Together, these projects cover multiple layers of a production system.
- Go is the right language for this project:
  - Caddy and Traefik are written in Go.
  - Go fits well for networking, concurrency, and infrastructure tooling.
- Kubernetes and Terraform deployment makes it resume-ready for infrastructure roles.
- Benchmarking against Nginx creates a strong performance story with real numbers.

---

## 2. Feature Scope

Build the features in strict order. Each feature should be independently useful and testable before moving to the next.

| Feature | Description | Effort |
|---|---|---:|
| HTTP reverse proxy | Forward requests to upstream servers and stream responses back | 1 day |
| Dynamic routing | Route by path prefix, host header, or HTTP method | 1 day |
| Round-robin load balancing | Distribute requests evenly across upstream pool | 1 day |
| Least-connections load balancing | Route to upstream with fewest active connections | 1 day |
| Weighted load balancing | Assign traffic weight per upstream, for example 70/30 split | 0.5 days |
| Health checks | Periodically ping upstreams and remove unhealthy ones from the pool | 1 day |
| Rate limiting | Token bucket per client IP with configurable req/sec limit | 1.5 days |
| Circuit breaker | Stop routing to failing upstreams with half-open recovery | 2 days |
| Prometheus metrics | Request count, latency histogram, upstream health, and error rate | 1 day |
| Config file | YAML-based route and upstream config with hot-reload on SIGHUP | 1 day |
| Kubernetes + Terraform deploy | Deploy to local Kubernetes cluster and provision with Terraform | 2 days |
| Nginx benchmark | Compare throughput and latency percentiles under load | 1 day |

**Total estimated time:** 3–4 weeks of focused work.

---

## 3. Architecture

The proxy is built as a pipeline of middleware handlers.

Each request passes through the pipeline in order. This is the same pattern used by Caddy and Chi: clean, testable, and easy to extend.

```text
Incoming Request
        |
        v
┌─────────────────────────────────────────┐
│              Listener (TCP)             │
└──────────────────┬──────────────────────┘
                   |
        ┌──────────v──────────┐
        │   Middleware Chain   │
        │                      │
        │ 1. Rate Limiter      │
        │ 2. Router            │
        │ 3. Circuit Breaker   │
        │ 4. Load Balancer     │
        │ 5. Reverse Proxy     │
        │ 6. Metrics Recorder  │
        └──────────┬──────────┘
                   |
        ┌──────────v──────────┐
        │    Upstream Pool     │
        │                      │
        │ server-1:8081        │
        │ server-2:8082        │
        │ server-3:8083        │
        └─────────────────────┘
```
