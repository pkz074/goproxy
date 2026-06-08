package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkz074/goproxy/internal/proxy"
)

func MatchesByPathPrefix(t *testing.T) {
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

func NotMatchWrongPath(t *testing.T) {
	routes := []proxy.Route{
		{PathPrefix: "/api", UpstreamURL: "http://api-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/admin", http.NoBody)
	_, ok := proxy.MatchRoute(routes, request)
	if ok {
		t.Fatal("expected route not to match")
	}
}

func EmptyHostMatchesAnyHost(t *testing.T) {
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

func RequiresHostWhenConfigured(t *testing.T) {
	routes := []proxy.Route{
		{Host: "admin.local", PathPrefix: "/", UpstreamURL: "http://admin-service"},
	}
	request := httptest.NewRequest(http.MethodGet, "http://api-local/users", http.NoBody)
	_, ok := proxy.MatchRoute(routes, request)

	if ok {
		t.Fatal("expected route to not match")
	}
}

func EmptyMethodMatchesAnyMethod(t *testing.T) {
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

func RequiresMethodWhenConfigured(t *testing.T) {
	routes := []proxy.Route{
		{Method: http.MethodPost, PathPrefix: "/orders", UpstreamURL: "http://orders-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/orders", http.NoBody)
	_, ok := proxy.MatchRoute(routes, request)

	if ok {
		t.Fatal("expected route not to match")
	}
}

func RouteLongestPathPrefixWins(t *testing.T) {
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

func ReturnsFalseWhenNoRouteMatches(t *testing.T) {
	routes := []proxy.Route{
		{Host: "admin.local", Method: http.MethodPost, PathPrefix: "/admin", UpstreamURL: "http://admin-service"},
	}

	request := httptest.NewRequest(http.MethodGet, "http://api.local/api/users", http.NoBody)
	route, ok := proxy.MatchRoute(routes, request)

	if ok {
		t.Fatalf("expected no route, got %+v", route)
	}
}

func ProxyForwardsToMatchedUpstream(t *testing.T) {
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

func NotFoundWhenNoRouteMatches(t *testing.T) {
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
