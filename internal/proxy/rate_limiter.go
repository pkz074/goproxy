package proxy

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond float64
	Burst             int
	IdleTimeout       time.Duration
	CleanupInterval   time.Duration
}

type RateLimiter struct {
	mu          sync.Mutex
	config      RateLimitConfig
	clients     map[string]*clientBucket
	now         func() time.Time
	lastCleanup time.Time
}

type clientBucket struct {
	tokens   float64
	lastSeen time.Time
}

var (
	ErrInvalidRate  = errors.New("rate limit requests per second must be greater than zero")
	ErrInvalidBurst = errors.New("rate limit burst must be greater than zero")
)

func NewRateLimiter(config RateLimitConfig) (*RateLimiter, error) {
	return newRateLimiter(config, time.Now)
}

func NewRateLimitedHandler(next http.Handler, limiter *RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limiter == nil || limiter.Allow(r) {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "too many requests", http.StatusTooManyRequests)
	})
}

func (r *RateLimiter) Allow(request *http.Request) bool {
	clientIP := clientIP(request.RemoteAddr)
	if clientIP == "" {
		clientIP = request.RemoteAddr
	}

	return r.AllowClient(clientIP)
}

func (r *RateLimiter) AllowClient(clientID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.cleanup(now)

	bucket, ok := r.clients[clientID]
	if !ok {
		r.clients[clientID] = &clientBucket{
			tokens:   float64(r.config.Burst - 1),
			lastSeen: now,
		}
		return true
	}

	elapsed := now.Sub(bucket.lastSeen).Seconds()
	bucket.tokens = min(float64(r.config.Burst), bucket.tokens+elapsed*r.config.RequestsPerSecond)
	bucket.lastSeen = now

	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--
	return true
}

func (r *RateLimiter) ClientCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanup(r.now())
	return len(r.clients)
}

func newRateLimiter(config RateLimitConfig, now func() time.Time) (*RateLimiter, error) {
	config = normalizeRateLimitConfig(config)

	if config.RequestsPerSecond <= 0 {
		return nil, ErrInvalidRate
	}

	if config.Burst <= 0 {
		return nil, ErrInvalidBurst
	}

	return &RateLimiter{
		config:      config,
		clients:     make(map[string]*clientBucket),
		now:         now,
		lastCleanup: now(),
	}, nil
}

func normalizeRateLimitConfig(config RateLimitConfig) RateLimitConfig {
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 5 * time.Minute
	}

	if config.CleanupInterval <= 0 {
		config.CleanupInterval = time.Minute
	}

	return config
}

func (r *RateLimiter) cleanup(now time.Time) {
	if r.config.IdleTimeout <= 0 {
		return
	}

	if now.Sub(r.lastCleanup) < r.config.CleanupInterval {
		return
	}

	for clientID, bucket := range r.clients {
		if now.Sub(bucket.lastSeen) > r.config.IdleTimeout {
			delete(r.clients, clientID)
		}
	}

	r.lastCleanup = now
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}
