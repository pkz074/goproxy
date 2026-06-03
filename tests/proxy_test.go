package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pkz074/goproxy/internal/proxy"
)

func TestProxyForwardsRequestToUpstream(t *testing.T) {
	type upstreamRequest struct {
		method    string
		path      string
		query     string
		requestID string
		body      string
	}

	received := make(chan upstreamRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		received <- upstreamRequest{
			method:    r.Method,
			path:      r.URL.Path,
			query:     r.URL.RawQuery,
			requestID: r.Header.Get("X-Request-ID"),
			body:      string(body),
		}

		w.Header().Set("X-Upstream", "test-backend")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	t.Cleanup(upstream.Close)

	handler, err := proxy.New(upstream.URL)
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"http://proxy.local/api/users?active=true",
		strings.NewReader(`{"name":"ada"}`),
	)
	request.Header.Set("X-Request-ID", "req-123")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	result := response.Result()
	t.Cleanup(func() { _ = result.Body.Close() })

	if result.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", result.StatusCode, http.StatusCreated)
	}

	if result.Header.Get("X-Upstream") != "test-backend" {
		t.Fatalf("upstream response header was not returned")
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if string(body) != "created" {
		t.Fatalf("body = %q, want created", string(body))
	}

	got := <-received
	if got.method != http.MethodPost {
		t.Fatalf("method = %s, want %s", got.method, http.MethodPost)
	}

	if got.path != "/api/users" {
		t.Fatalf("path = %s, want /api/users", got.path)
	}

	if got.query != "active=true" {
		t.Fatalf("query = %s, want active=true", got.query)
	}

	if got.requestID != "req-123" {
		t.Fatalf("X-Request-ID header was not forwarded")
	}

	if got.body != `{"name":"ada"}` {
		t.Fatalf("body = %q, want JSON payload", got.body)
	}
}

func TestProxyRejectsInvalidUpstream(t *testing.T) {
	tests := []string{
		"",
		"localhost:8080",
		"ftp://localhost:8080",
	}

	for _, upstream := range tests {
		t.Run(upstream, func(t *testing.T) {
			if _, err := proxy.New(upstream); err == nil {
				t.Fatal("expected invalid upstream to return an error")
			}
		})
	}
}

func TestProxyReturnsBadGatewayWhenUpstreamIsUnavailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	upstreamURL := upstream.URL
	upstream.Close()

	handler, err := proxy.New(upstreamURL)
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/", http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
}
