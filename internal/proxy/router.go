package proxy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type Route struct {
	Host           string
	Method         string
	PathPrefix     string
	UpstreamURL    string
	Upstreams      []WeightedUpstream
	Strategy       BalancingStrategy
	HealthCheck    HealthCheckConfig
	RateLimit      RateLimitConfig
	CircuitBreaker CircuitBreakerConfig
}

type RoutedProxy struct {
	routes  []routeRuntime
	proxies map[string]*Proxy
}

type routeRuntime struct {
	route    Route
	balancer routeBalancer
	health   *HealthTable
	limiter  *RateLimiter
	breakers map[string]*CircuitBreaker
	attempts int
}

func NewRouted(routes []Route) (*RoutedProxy, error) {
	for _, route := range routes {
		if route.HealthCheck.Enabled {
			return nil, ErrContextRequired
		}
	}

	return newRouted(context.Background(), routes)
}

func NewRoutedWithContext(ctx context.Context, routes []Route) (*RoutedProxy, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}

	return newRouted(ctx, routes)
}

func newRouted(ctx context.Context, routes []Route) (*RoutedProxy, error) {
	routeCopy := copyRoutes(routes)
	runtimes := make([]routeRuntime, 0, len(routeCopy))
	proxies := make(map[string]*Proxy)
	checkers := make([]*HealthChecker, 0, len(routeCopy))

	for _, route := range routeCopy {
		upstreams, err := routeUpstreams(route)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", route.PathPrefix, err)
		}

		balancer, err := newRouteBalancer(route, upstreams)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", route.PathPrefix, err)
		}

		plain := plainUpstreams(upstreams)
		health := NewHealthTable(plain)
		if route.HealthCheck.Enabled {
			checker := NewHealthChecker(plain, route.HealthCheck, health)
			checkers = append(checkers, checker)
		}

		var limiter *RateLimiter
		if route.RateLimit.Enabled {
			limiter, err = NewRateLimiter(route.RateLimit)
			if err != nil {
				return nil, fmt.Errorf("route %q: %w", route.PathPrefix, err)
			}
		}

		breakers := make(map[string]*CircuitBreaker, len(plain))
		if route.CircuitBreaker.Enabled {
			for _, upstream := range plain {
				breaker, err := NewCircuitBreaker(route.CircuitBreaker)
				if err != nil {
					return nil, fmt.Errorf("route %q: %w", route.PathPrefix, err)
				}

				breakers[upstream.URL] = breaker
			}
		}

		for _, upstream := range upstreams {
			if _, ok := proxies[upstream.Upstream.URL]; ok {
				continue
			}

			proxy, err := New(upstream.Upstream.URL)
			if err != nil {
				return nil, fmt.Errorf("create proxy for route %q: %w", route.PathPrefix, err)
			}

			proxies[upstream.Upstream.URL] = proxy
		}

		runtimes = append(runtimes, routeRuntime{
			route:    route,
			balancer: balancer,
			health:   health,
			limiter:  limiter,
			breakers: breakers,
			attempts: len(upstreams),
		})
	}

	for _, checker := range checkers {
		go checker.Run(ctx)
	}

	return &RoutedProxy{
		routes:  runtimes,
		proxies: proxies,
	}, nil
}

func MatchRoute(routes []Route, r *http.Request) (*Route, bool) {
	var best *Route

	for i := range routes {
		route := &routes[i]

		if route.Host != "" && route.Host != r.Host {
			continue
		}

		if route.Method != "" && route.Method != r.Method {
			continue
		}

		if !strings.HasPrefix(r.URL.Path, route.PathPrefix) {
			continue
		}

		if best == nil || len(route.PathPrefix) > len(best.PathPrefix) {
			best = route
		}
	}

	if best == nil {
		return nil, false
	}

	return best, true
}

func (p *RoutedProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	runtime, ok := p.matchRoute(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if runtime.limiter != nil && !runtime.limiter.Allow(r) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	upstream, release, err := runtime.acquireAvailable()
	if err != nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer release()

	upstreamProxy, ok := p.proxies[upstream.URL]
	if !ok {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	recorder := &statusRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}

	upstreamProxy.ServeHTTP(recorder, r)
	runtime.recordResult(upstream, recorder.status)
}

func (r *routeRuntime) acquireAvailable() (Upstream, func(), error) {
	for range r.attempts {
		upstream, release, err := r.balancer.Acquire()
		if err != nil {
			return Upstream{}, func() {}, err
		}

		breaker := r.breakers[upstream.URL]
		if r.health.IsHealthy(upstream) && (breaker == nil || breaker.Allow()) {
			return upstream, release, nil
		}

		release()
	}

	return Upstream{}, func() {}, ErrNoUpstreams
}

func (r *routeRuntime) recordResult(upstream Upstream, status int) {
	breaker := r.breakers[upstream.URL]
	if breaker == nil {
		return
	}

	if status >= http.StatusInternalServerError {
		breaker.RecordFailure()
		return
	}

	breaker.RecordSuccess()
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(status int) {
	if s.wroteHeader {
		return
	}

	s.wroteHeader = true
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func (s *statusRecorder) Flush() {
	flusher, ok := s.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}

	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}

	flusher.Flush()
}

func (p *RoutedProxy) matchRoute(r *http.Request) (*routeRuntime, bool) {
	var best *routeRuntime

	for i := range p.routes {
		runtime := &p.routes[i]
		route := &runtime.route

		if route.Host != "" && route.Host != r.Host {
			continue
		}

		if route.Method != "" && route.Method != r.Method {
			continue
		}

		if !strings.HasPrefix(r.URL.Path, route.PathPrefix) {
			continue
		}

		if best == nil || len(route.PathPrefix) > len(best.route.PathPrefix) {
			best = runtime
		}
	}

	if best == nil {
		return nil, false
	}

	return best, true
}

func copyRoutes(routes []Route) []Route {
	copied := make([]Route, len(routes))
	for i, route := range routes {
		copied[i] = route
		copied[i].Upstreams = append([]WeightedUpstream(nil), route.Upstreams...)
	}

	return copied
}

func routeUpstreams(route Route) ([]WeightedUpstream, error) {
	hasSingleUpstream := route.UpstreamURL != ""
	hasPool := len(route.Upstreams) > 0

	if hasSingleUpstream == hasPool {
		return nil, ErrInvalidRoute
	}

	if hasSingleUpstream {
		return []WeightedUpstream{
			{Upstream: Upstream{URL: route.UpstreamURL}, Weight: 1},
		}, nil
	}

	return append([]WeightedUpstream(nil), route.Upstreams...), nil
}

func newRouteBalancer(route Route, upstreams []WeightedUpstream) (routeBalancer, error) {
	strategy := route.Strategy
	if strategy == "" {
		strategy = StrategyRoundRobin
	}

	switch strategy {
	case StrategyRoundRobin:
		balancer, err := NewRoundRobin(plainUpstreams(upstreams))
		if err != nil {
			return nil, err
		}

		return &roundRobinRouteBalancer{balancer: balancer}, nil
	case StrategyLeastConnections:
		balancer, err := NewLeastConnections(plainUpstreams(upstreams))
		if err != nil {
			return nil, err
		}

		return &leastConnectionsRouteBalancer{balancer: balancer}, nil
	case StrategyWeightedRoundRobin:
		balancer, err := NewWeightedRoundRobin(upstreams)
		if err != nil {
			return nil, err
		}

		return &weightedRoundRobinRouteBalancer{balancer: balancer}, nil
	default:
		return nil, ErrUnknownStrategy
	}
}

func plainUpstreams(upstreams []WeightedUpstream) []Upstream {
	plain := make([]Upstream, len(upstreams))
	for i, upstream := range upstreams {
		plain[i] = upstream.Upstream
	}

	return plain
}
