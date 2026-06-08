package proxy

import (
	"net/http"
	"strings"
)

type Route struct {
	Host        string
	Method      string
	PathPrefix  string
	UpstreamURL string
}

type RoutedProxy struct {
	routes  []Route
	proxies map[string]*Proxy
}

func NewRouted(routes []Route) (*RoutedProxy, error) {
	proxies := make(map[string]*Proxy)

	for _, route := range routes {
		if _, ok := proxies[route.UpstreamURL]; ok {
			continue
		}

		proxy, err := New(route.UpstreamURL)
		if err != nil {
			return nil, err
		}

		proxies[route.UpstreamURL] = proxy
	}

	return &RoutedProxy{
		routes:  routes,
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
	route, ok := MatchRoute(p.routes, r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	p.proxies[route.UpstreamURL].ServeHTTP(w, r)
}
