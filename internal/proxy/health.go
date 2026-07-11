package proxy

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type HealthCheckConfig struct {
	Enabled            bool
	Path               string
	Interval           time.Duration
	Timeout            time.Duration
	HealthyThreshold   int
	UnhealthyThreshold int
}

type HealthTable struct {
	mu      sync.RWMutex
	healthy map[string]bool
}

func NewHealthTable(upstreams []Upstream) *HealthTable {
	healthy := make(map[string]bool, len(upstreams))
	for _, upstream := range upstreams {
		healthy[upstream.URL] = true
	}

	return &HealthTable{healthy: healthy}
}

func (h *HealthTable) IsHealthy(upstream Upstream) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	healthy, ok := h.healthy[upstream.URL]
	return !ok || healthy
}

func (h *HealthTable) SetHealthy(upstream Upstream, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.healthy[upstream.URL] = healthy
}

func (h *HealthTable) Snapshot() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	snapshot := make(map[string]bool, len(h.healthy))
	for upstream, healthy := range h.healthy {
		snapshot[upstream] = healthy
	}

	return snapshot
}

type HealthChecker struct {
	upstreams []Upstream
	config    HealthCheckConfig
	table     *HealthTable
	client    *http.Client

	mu       sync.Mutex
	success  map[string]int
	failures map[string]int
}

func NewHealthChecker(upstreams []Upstream, config HealthCheckConfig, table *HealthTable) *HealthChecker {
	config = normalizeHealthCheckConfig(config)

	return &HealthChecker{
		upstreams: append([]Upstream(nil), upstreams...),
		config:    config,
		table:     table,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		success:  make(map[string]int, len(upstreams)),
		failures: make(map[string]int, len(upstreams)),
	}
}

func (h *HealthChecker) Run(ctx context.Context) {
	h.CheckOnce(ctx)

	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.CheckOnce(ctx)
		}
	}
}

func (h *HealthChecker) CheckOnce(ctx context.Context) {
	for _, upstream := range h.upstreams {
		healthy := h.check(ctx, upstream)
		h.record(upstream, healthy)
	}
}

func (h *HealthChecker) check(ctx context.Context, upstream Upstream) bool {
	target, err := healthCheckURL(upstream.URL, h.config.Path)
	if err != nil {
		return false
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return false
	}

	response, err := h.client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()

	return response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusBadRequest
}

func (h *HealthChecker) record(upstream Upstream, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if healthy {
		h.success[upstream.URL]++
		h.failures[upstream.URL] = 0

		if h.success[upstream.URL] >= h.config.HealthyThreshold {
			h.table.SetHealthy(upstream, true)
		}

		return
	}

	h.failures[upstream.URL]++
	h.success[upstream.URL] = 0

	if h.failures[upstream.URL] >= h.config.UnhealthyThreshold {
		h.table.SetHealthy(upstream, false)
	}
}

func normalizeHealthCheckConfig(config HealthCheckConfig) HealthCheckConfig {
	if config.Path == "" {
		config.Path = "/healthz"
	}

	if config.Interval <= 0 {
		config.Interval = 5 * time.Second
	}

	if config.Timeout <= 0 {
		config.Timeout = time.Second
	}

	if config.HealthyThreshold <= 0 {
		config.HealthyThreshold = 1
	}

	if config.UnhealthyThreshold <= 0 {
		config.UnhealthyThreshold = 1
	}

	return config
}

func healthCheckURL(rawUpstream string, path string) (string, error) {
	upstream, err := url.Parse(rawUpstream)
	if err != nil {
		return "", err
	}

	checkURL := *upstream
	checkURL.Path = path
	checkURL.RawQuery = ""
	checkURL.Fragment = ""

	return checkURL.String(), nil
}
