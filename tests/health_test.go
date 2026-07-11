package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pkz074/goproxy/internal/proxy"
)

func TestHealthCheckerMarksUnhealthyAfterThreshold(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)

	table := proxy.NewHealthTable([]proxy.Upstream{{URL: upstream.URL}})
	checker := proxy.NewHealthChecker(
		[]proxy.Upstream{{URL: upstream.URL}},
		proxy.HealthCheckConfig{
			Path:               "/healthz",
			UnhealthyThreshold: 2,
		},
		table,
	)

	checker.CheckOnce(context.Background())
	if !table.IsHealthy(proxy.Upstream{URL: upstream.URL}) {
		t.Fatal("upstream should stay healthy before threshold")
	}

	checker.CheckOnce(context.Background())
	if table.IsHealthy(proxy.Upstream{URL: upstream.URL}) {
		t.Fatal("upstream should be unhealthy after threshold")
	}
}

func TestHealthCheckerMarksHealthyAfterThreshold(t *testing.T) {
	healthy := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !healthy {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	target := proxy.Upstream{URL: upstream.URL}
	table := proxy.NewHealthTable([]proxy.Upstream{target})
	checker := proxy.NewHealthChecker(
		[]proxy.Upstream{target},
		proxy.HealthCheckConfig{
			Path:               "/healthz",
			HealthyThreshold:   2,
			UnhealthyThreshold: 1,
		},
		table,
	)

	checker.CheckOnce(context.Background())
	if table.IsHealthy(target) {
		t.Fatal("upstream should be unhealthy after failed check")
	}

	healthy = true
	checker.CheckOnce(context.Background())
	if table.IsHealthy(target) {
		t.Fatal("upstream should stay unhealthy before healthy threshold")
	}

	checker.CheckOnce(context.Background())
	if !table.IsHealthy(target) {
		t.Fatal("upstream should be healthy after threshold")
	}
}

func TestHealthCheckerRunStopsWhenContextIsCanceled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	ctx, cancel := context.WithCancel(context.Background())
	table := proxy.NewHealthTable([]proxy.Upstream{{URL: upstream.URL}})
	checker := proxy.NewHealthChecker(
		[]proxy.Upstream{{URL: upstream.URL}},
		proxy.HealthCheckConfig{
			Path:     "/healthz",
			Interval: time.Millisecond,
		},
		table,
	)

	done := make(chan struct{})
	go func() {
		defer close(done)
		checker.Run(ctx)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("health checker did not stop after context cancellation")
	}
}

func TestHealthTableSnapshotReturnsCopy(t *testing.T) {
	upstream := proxy.Upstream{URL: "http://upstream-a"}
	table := proxy.NewHealthTable([]proxy.Upstream{upstream})

	snapshot := table.Snapshot()
	snapshot[upstream.URL] = false

	if !table.IsHealthy(upstream) {
		t.Fatal("mutating snapshot should not change table")
	}
}
