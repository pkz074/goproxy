package proxy

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstThenRejects(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             2,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	if !limiter.AllowClient("client-a") {
		t.Fatal("first request should be allowed")
	}

	if !limiter.AllowClient("client-a") {
		t.Fatal("second request should be allowed")
	}

	if limiter.AllowClient("client-a") {
		t.Fatal("third request should be rejected")
	}
}

func TestRateLimiterRefillsTokens(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 10,
		Burst:             1,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	if !limiter.AllowClient("client-a") {
		t.Fatal("first request should be allowed")
	}

	if limiter.AllowClient("client-a") {
		t.Fatal("second request should be rejected")
	}

	clock.Advance(100 * time.Millisecond)

	if !limiter.AllowClient("client-a") {
		t.Fatal("request should be allowed after refill")
	}
}

func TestRateLimiterIsolatesClients(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             1,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	if !limiter.AllowClient("client-a") {
		t.Fatal("client-a first request should be allowed")
	}

	if limiter.AllowClient("client-a") {
		t.Fatal("client-a second request should be rejected")
	}

	if !limiter.AllowClient("client-b") {
		t.Fatal("client-b first request should be allowed")
	}
}

func TestRateLimiterUsesRemoteAddrHost(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             1,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	first := httptest.NewRequest(http.MethodGet, "http://proxy.local/api", http.NoBody)
	first.RemoteAddr = "192.0.2.10:1234"
	first.Header.Set("X-Forwarded-For", "198.51.100.10")

	second := httptest.NewRequest(http.MethodGet, "http://proxy.local/api", http.NoBody)
	second.RemoteAddr = "192.0.2.10:5678"
	second.Header.Set("X-Forwarded-For", "203.0.113.10")

	if !limiter.Allow(first) {
		t.Fatal("first request should be allowed")
	}

	if limiter.Allow(second) {
		t.Fatal("second request from same RemoteAddr host should be rejected")
	}
}

func TestRateLimiterRemovesIdleClients(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		IdleTimeout:       time.Second,
		CleanupInterval:   time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	if !limiter.AllowClient("client-a") {
		t.Fatal("first request should be allowed")
	}

	if count := limiter.ClientCount(); count != 1 {
		t.Fatalf("client count = %d, want 1", count)
	}

	clock.Advance(2 * time.Second)

	if count := limiter.ClientCount(); count != 0 {
		t.Fatalf("client count = %d, want 0", count)
	}
}

func TestRateLimiterWaitsForCleanupInterval(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		IdleTimeout:       time.Second,
		CleanupInterval:   5 * time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	if !limiter.AllowClient("client-a") {
		t.Fatal("first request should be allowed")
	}

	clock.Advance(2 * time.Second)

	if count := limiter.ClientCount(); count != 1 {
		t.Fatalf("client count before cleanup interval = %d, want 1", count)
	}

	clock.Advance(4 * time.Second)

	if count := limiter.ClientCount(); count != 0 {
		t.Fatalf("client count after cleanup interval = %d, want 0", count)
	}
}

func TestNewRateLimiterRejectsInvalidConfig(t *testing.T) {
	limiter, err := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 0,
		Burst:             1,
	})

	if !errors.Is(err, ErrInvalidRate) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidRate)
	}

	if limiter != nil {
		t.Fatalf("limiter = %#v, want nil", limiter)
	}

	limiter, err = NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             0,
	})

	if !errors.Is(err, ErrInvalidBurst) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidBurst)
	}

	if limiter != nil {
		t.Fatalf("limiter = %#v, want nil", limiter)
	}
}

func TestRateLimitedHandlerReturnsTooManyRequests(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             1,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	handler := NewRateLimitedHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}), limiter)

	first := httptest.NewRequest(http.MethodGet, "http://proxy.local/api", http.NoBody)
	first.RemoteAddr = "192.0.2.10:1234"
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, first)

	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusOK)
	}

	second := httptest.NewRequest(http.MethodGet, "http://proxy.local/api", http.NoBody)
	second.RemoteAddr = "192.0.2.10:5678"
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, second)

	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", secondResponse.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	clock := newFakeClock()
	limiter, err := newRateLimiter(RateLimitConfig{
		RequestsPerSecond: 1,
		Burst:             100,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	const requests = 200
	results := make(chan bool, requests)
	var wg sync.WaitGroup

	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- limiter.AllowClient("client-a")
		}()
	}

	wg.Wait()
	close(results)

	allowed := 0
	for result := range results {
		if result {
			allowed++
		}
	}

	if allowed != 100 {
		t.Fatalf("allowed requests = %d, want 100", allowed)
	}
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(0, 0)}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.now
}

func (f *fakeClock) Advance(duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.now = f.now.Add(duration)
}
