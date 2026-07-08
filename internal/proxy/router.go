package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

type Route struct {
	Host        string
	Method      string
	PathPrefix  string
	UpstreamURL string
	Upstreams   []WeightedUpstream
	Strategy    BalancingStrategy
}

type RoutedProxy struct {
	routes  []routeRuntime
	proxies map[string]*Proxy
}

type routeRuntime struct {
	route    Route
	balancer routeBalancer
}

func NewRouted(routes []Route) (*RoutedProxy, error) {
	routeCopy := copyRoutes(routes)
	runtimes := make([]routeRuntime, 0, len(routeCopy))
	proxies := make(map[string]*Proxy)

	for _, route := range routeCopy {
		upstreams, err := routeUpstreams(route)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", route.PathPrefix, err)
		}

		balancer, err := newRouteBalancer(route, upstreams)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", route.PathPrefix, err)
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
		})
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

	upstream, release, err := runtime.balancer.Acquire()
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

	upstreamProxy.ServeHTTP(w, r)
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
		plain := make([]Upstream, len(upstreams))
		for i, upstream := range upstreams {
			plain[i] = upstream.Upstream
		}

		balancer, err := NewRoundRobin(plain)
		if err != nil {
			return nil, err
		}

		return &roundRobinRouteBalancer{balancer: balancer}, nil
	case StrategyLeastConnections:
		plain := make([]Upstream, len(upstreams))
		for i, upstream := range upstreams {
			plain[i] = upstream.Upstream
		}

		balancer, err := NewLeastConnections(plain)
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
