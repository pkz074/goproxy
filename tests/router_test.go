package tests

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/pkz074/goproxy/internal/proxy"
)

func TestMatchesByPathPrefix(t *testing.T) {
	routes := []proxy.Route{
		{PathPrefix: "/api", UpstreamURL: "http://api-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/api/users", http.NoBody)
	route, ok := proxy.MatchRoute(routes, request)

	if !ok {
		t.Fatal("expected route to match")
	}

	if route.UpstreamURL != "http://api-service" {
		t.Fatalf("upstream = %s, want http://api-service", route.UpstreamURL)
	}
}

func TestNotMatchWrongPath(t *testing.T) {
	routes := []proxy.Route{
		{PathPrefix: "/api", UpstreamURL: "http://api-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/admin", http.NoBody)
	_, ok := proxy.MatchRoute(routes, request)
	if ok {
		t.Fatal("expected route not to match")
	}
}

func TestEmptyHostMatchesAnyHost(t *testing.T) {
	routes := []proxy.Route{
		{Host: "", PathPrefix: "/api", UpstreamURL: "http://api-service"},
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.local/api/users", http.NoBody)
	route, ok := proxy.MatchRoute(routes, request)

	if !ok {
		t.Fatal("expected route to match")
	}
	if route.UpstreamURL != "http://api-service" {
		t.Fatalf("upstream = %s, want http://api-service", route.UpstreamURL)
	}
}

func TestRequiresHostWhenConfigured(t *testing.T) {
	routes := []proxy.Route{
		{Host: "admin.local", PathPrefix: "/", UpstreamURL: "http://admin-service"},
	}
	request := httptest.NewRequest(http.MethodGet, "http://api-local/users", http.NoBody)
	_, ok := proxy.MatchRoute(routes, request)

	if ok {
		t.Fatal("expected route to not match")
	}
}

func TestEmptyMethodMatchesAnyMethod(t *testing.T) {
	routes := []proxy.Route{
		{Method: "", PathPrefix: "/orders", UpstreamURL: "http://orders-service"},
	}

	request := httptest.NewRequest(http.MethodPost, "http://proxy.local/orders", http.NoBody)
	route, ok := proxy.MatchRoute(routes, request)

	if !ok {
		t.Fatal("expected route to match")
	}

	if route.UpstreamURL != "http://orders-service" {
		t.Fatalf("upstream = %s, want http://orders-service", route.UpstreamURL)
	}
}

func TestRequiresMethodWhenConfigured(t *testing.T) {
	routes := []proxy.Route{
		{Method: http.MethodPost, PathPrefix: "/orders", UpstreamURL: "http://orders-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/orders", http.NoBody)
	_, ok := proxy.MatchRoute(routes, request)

	if ok {
		t.Fatal("expected route not to match")
	}
}

func TestRouteLongestPathPrefixWins(t *testing.T) {
	routes := []proxy.Route{
		{PathPrefix: "/api", UpstreamURL: "http://api-service"},
		{PathPrefix: "/api/users", UpstreamURL: "http://users-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/api/users/123", http.NoBody)
	route, ok := proxy.MatchRoute(routes, request)

	if !ok {
		t.Fatal("expected route to match")
	}

	if route.UpstreamURL != "http://users-service" {
		t.Fatalf("upstream = %s, want http://users-service", route.UpstreamURL)
	}
}

func TestReturnsFalseWhenNoRouteMatches(t *testing.T) {
	routes := []proxy.Route{
		{Host: "admin.local", Method: http.MethodPost, PathPrefix: "/admin", UpstreamURL: "http://admin-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://api.local/api/users", http.NoBody)
	route, ok := proxy.MatchRoute(routes, request)

	if ok {
		t.Fatalf("expected no route, got %+v", route)
	}
}

func TestProxyForwardsToMatchedUpstream(t *testing.T) {
	users := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("users"))
	}))
	t.Cleanup(users.Close)

	orders := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("orders"))
	}))
	t.Cleanup(orders.Close)

	handler, err := proxy.NewRouted([]proxy.Route{
		{PathPrefix: "/users", UpstreamURL: users.URL},
		{PathPrefix: "/orders", UpstreamURL: orders.URL},
	})
	if err != nil {
		t.Fatalf("create routed proxy: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/orders/123", http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	result := response.Result()
	t.Cleanup(func() { _ = result.Body.Close() })

	if result.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", result.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if string(body) != "orders" {
		t.Fatalf("body = %q, want orders", string(body))
	}
}

func TestNotFoundWhenNoRouteMatches(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("upstream"))
	}))
	t.Cleanup(upstream.Close)

	handler, err := proxy.NewRouted([]proxy.Route{
		{PathPrefix: "/api", UpstreamURL: upstream.URL},
	})
	if err != nil {
		t.Fatalf("create routed proxy: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/admin", http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestNewRoutedCopiesRouteSlice(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("original"))
	}))
	t.Cleanup(upstream.Close)

	routes := []proxy.Route{
		{PathPrefix: "/api", UpstreamURL: upstream.URL},
	}

	handler, err := proxy.NewRouted(routes)
	if err != nil {
		t.Fatalf("create routed proxy: %v", err)
	}

	routes[0].PathPrefix = "/changed"

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/api/users", http.NoBody)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	if body := response.Body.String(); body != "original" {
		t.Fatalf("body = %q, want %q", body, "original")
	}
}

func TestRoutedProxyRoundRobinLoadBalancesUpstreamPool(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first"))
	}))
	t.Cleanup(first.Close)

	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("second"))
	}))
	t.Cleanup(second.Close)

	handler, err := proxy.NewRouted([]proxy.Route{
		{
			PathPrefix: "/api",
			Strategy:   proxy.StrategyRoundRobin,
			Upstreams: []proxy.WeightedUpstream{
				{Upstream: proxy.Upstream{URL: first.URL}},
				{Upstream: proxy.Upstream{URL: second.URL}},
			},
		},
	})
	if err != nil {
		t.Fatalf("create routed proxy: %v", err)
	}

	want := []string{"first", "second", "first", "second"}
	for i, wantBody := range want {
		body := serveRoutedRequest(t, handler, "/api/users")
		if body != wantBody {
			t.Fatalf("request %d body = %q, want %q", i+1, body, wantBody)
		}
	}
}

func TestRoutedProxyWeightedRoundRobinLoadBalancesUpstreamPool(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first"))
	}))
	t.Cleanup(first.Close)

	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("second"))
	}))
	t.Cleanup(second.Close)

	handler, err := proxy.NewRouted([]proxy.Route{
		{
			PathPrefix: "/api",
			Strategy:   proxy.StrategyWeightedRoundRobin,
			Upstreams: []proxy.WeightedUpstream{
				{Upstream: proxy.Upstream{URL: first.URL}, Weight: 3},
				{Upstream: proxy.Upstream{URL: second.URL}, Weight: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("create routed proxy: %v", err)
	}

	want := []string{"first", "first", "second", "first"}
	for i, wantBody := range want {
		body := serveRoutedRequest(t, handler, "/api/users")
		if body != wantBody {
			t.Fatalf("request %d body = %q, want %q", i+1, body, wantBody)
		}
	}
}

func TestRoutedProxyLeastConnectionsReleasesAfterRequest(t *testing.T) {
	firstStarted := make(chan struct{})
	firstCanFinish := make(chan struct{})
	var once sync.Once

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		once.Do(func() {
			close(firstStarted)
		})
		<-firstCanFinish
		_, _ = w.Write([]byte("first"))
	}))
	t.Cleanup(first.Close)

	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("second"))
	}))
	t.Cleanup(second.Close)

	handler, err := proxy.NewRouted([]proxy.Route{
		{
			PathPrefix: "/api",
			Strategy:   proxy.StrategyLeastConnections,
			Upstreams: []proxy.WeightedUpstream{
				{Upstream: proxy.Upstream{URL: first.URL}},
				{Upstream: proxy.Upstream{URL: second.URL}},
			},
		},
	})
	if err != nil {
		t.Fatalf("create routed proxy: %v", err)
	}

	firstDone := make(chan string, 1)
	firstErr := make(chan error, 1)
	go func() {
		status, body, err := routedResponse(handler, "/api/users")
		if err != nil {
			firstErr <- err
			return
		}

		if status != http.StatusOK {
			firstErr <- errors.New("first request returned non-OK status")
			return
		}

		firstDone <- body
		firstErr <- nil
	}()

	<-firstStarted

	if body := serveRoutedRequest(t, handler, "/api/users"); body != "second" {
		t.Fatalf("overlapping request body = %q, want second", body)
	}

	close(firstCanFinish)

	if err := <-firstErr; err != nil {
		t.Fatalf("first request: %v", err)
	}

	if body := <-firstDone; body != "first" {
		t.Fatalf("first request body = %q, want first", body)
	}

	if body := serveRoutedRequest(t, handler, "/api/users"); body != "first" {
		t.Fatalf("post-release request body = %q, want first", body)
	}
}

func TestNewRoutedRejectsRouteWithoutUpstream(t *testing.T) {
	handler, err := proxy.NewRouted([]proxy.Route{
		{PathPrefix: "/api"},
	})

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, proxy.ErrInvalidRoute) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrInvalidRoute)
	}

	if handler != nil {
		t.Fatalf("handler = %#v, want nil", handler)
	}
}

func TestNewRoutedRejectsRouteWithSingleUpstreamAndPool(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("upstream"))
	}))
	t.Cleanup(upstream.Close)

	handler, err := proxy.NewRouted([]proxy.Route{
		{
			PathPrefix:  "/api",
			UpstreamURL: upstream.URL,
			Upstreams: []proxy.WeightedUpstream{
				{Upstream: proxy.Upstream{URL: upstream.URL}},
			},
		},
	})

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, proxy.ErrInvalidRoute) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrInvalidRoute)
	}

	if handler != nil {
		t.Fatalf("handler = %#v, want nil", handler)
	}
}

func TestNewRoutedRejectsUnknownStrategy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("upstream"))
	}))
	t.Cleanup(upstream.Close)

	handler, err := proxy.NewRouted([]proxy.Route{
		{
			PathPrefix: "/api",
			Strategy:   proxy.BalancingStrategy("unknown"),
			Upstreams: []proxy.WeightedUpstream{
				{Upstream: proxy.Upstream{URL: upstream.URL}},
			},
		},
	})

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, proxy.ErrUnknownStrategy) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrUnknownStrategy)
	}

	if handler != nil {
		t.Fatalf("handler = %#v, want nil", handler)
	}
}

func TestNewRoutedCopiesUpstreamPool(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("original"))
	}))
	t.Cleanup(upstream.Close)

	changed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("changed"))
	}))
	t.Cleanup(changed.Close)

	routes := []proxy.Route{
		{
			PathPrefix: "/api",
			Upstreams: []proxy.WeightedUpstream{
				{Upstream: proxy.Upstream{URL: upstream.URL}},
			},
		},
	}

	handler, err := proxy.NewRouted(routes)
	if err != nil {
		t.Fatalf("create routed proxy: %v", err)
	}

	routes[0].Upstreams[0].Upstream.URL = changed.URL

	if body := serveRoutedRequest(t, handler, "/api/users"); body != "original" {
		t.Fatalf("body = %q, want original", body)
	}
}

func serveRoutedRequest(t *testing.T, handler http.Handler, path string) string {
	t.Helper()

	status, body, err := routedResponse(handler, path)
	if err != nil {
		t.Fatalf("request routed proxy: %v", err)
	}

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}

	return body
}

func routedResponse(handler http.Handler, path string) (int, string, error) {
	request := httptest.NewRequest(http.MethodGet, "http://proxy.local"+path, http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	result := response.Result()
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return 0, "", err
	}

	return result.StatusCode, string(body), nil
}
