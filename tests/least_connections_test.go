package tests

import (
	"errors"
	"sync"
	"testing"

	"github.com/pkz074/goproxy/internal/proxy"
)

func TestLeastConnectionsSelectsFewestActive(t *testing.T) {
	balancer, err := proxy.NewLeastConnections([]proxy.Upstream{
		{URL: "http://upstream-a"},
		{URL: "http://upstream-b"},
	})
	if err != nil {
		t.Fatalf("create least-connections balancer: %v", err)
	}

	want := []string{
		"http://upstream-a",
		"http://upstream-b",
		"http://upstream-a",
	}

	for i, wantURL := range want {
		upstream, err := balancer.Acquire()
		if err != nil {
			t.Fatalf("Acquire call %d: %v", i+1, err)
		}

		if upstream.URL != wantURL {
			t.Fatalf("Acquire call %d returned %q, want %q", i+1, upstream.URL, wantURL)
		}
	}
}

func TestLeastConnectionsReleaseMakesUpstreamAvailable(t *testing.T) {
	balancer, err := proxy.NewLeastConnections([]proxy.Upstream{
		{URL: "http://upstream-a"},
		{URL: "http://upstream-b"},
	})
	if err != nil {
		t.Fatalf("create least-connections balancer: %v", err)
	}

	first, err := balancer.Acquire()
	if err != nil {
		t.Fatalf("acquire first upstream: %v", err)
	}

	_, err = balancer.Acquire()
	if err != nil {
		t.Fatalf("acquire second upstream: %v", err)
	}

	if err := balancer.Release(first); err != nil {
		t.Fatalf("release first upstream: %v", err)
	}

	next, err := balancer.Acquire()
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}

	if next.URL != first.URL {
		t.Fatalf("upstream after release = %q, want %q", next.URL, first.URL)
	}
}

func TestNewLeastConnectionsRejectsEmptyPool(t *testing.T) {
	balancer, err := proxy.NewLeastConnections(nil)

	if !errors.Is(err, proxy.ErrNoUpstreams) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrNoUpstreams)
	}

	if balancer != nil {
		t.Fatalf("balancer = %#v, want nil", balancer)
	}
}

func TestLeastConnectionsRejectsUnknownUpstream(t *testing.T) {
	balancer, err := proxy.NewLeastConnections([]proxy.Upstream{
		{URL: "http://upstream-a"},
	})
	if err != nil {
		t.Fatalf("create least-connections balancer: %v", err)
	}

	err = balancer.Release(proxy.Upstream{URL: "http://unknown-upstream"})

	if !errors.Is(err, proxy.ErrUnknownUpstream) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrUnknownUpstream)
	}
}

func TestLeastConnectionsPreventsNegativeActiveCount(t *testing.T) {
	balancer, err := proxy.NewLeastConnections([]proxy.Upstream{
		{URL: "http://upstream-a"},
	})
	if err != nil {
		t.Fatalf("create least-connections balancer: %v", err)
	}

	err = balancer.Release(proxy.Upstream{URL: "http://upstream-a"})

	if !errors.Is(err, proxy.ErrNoActiveRequests) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrNoActiveRequests)
	}
}

func TestLeastConnectionsCopiesUpstreamSlice(t *testing.T) {
	upstreams := []proxy.Upstream{
		{URL: "http://upstream-a"},
	}

	balancer, err := proxy.NewLeastConnections(upstreams)
	if err != nil {
		t.Fatalf("create least-connections balancer: %v", err)
	}

	upstreams[0].URL = "http://upstream-b"

	upstream, err := balancer.Acquire()
	if err != nil {
		t.Fatalf("acquire upstream: %v", err)
	}

	if upstream.URL != "http://upstream-a" {
		t.Fatalf("upstream = %q, want %q", upstream.URL, "http://upstream-a")
	}
}

func TestNewLeastConnectionsRejectsDuplicateUpstream(t *testing.T) {
	balancer, err := proxy.NewLeastConnections([]proxy.Upstream{
		{URL: "http://upstream-a"},
		{URL: "http://upstream-a"},
	})

	if !errors.Is(err, proxy.ErrDuplicateUpstream) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrDuplicateUpstream)
	}

	if balancer != nil {
		t.Fatalf("balancer = %#v, want nil", balancer)
	}
}

func TestLeastConnectionsConcurrentAcquireAndRelease(t *testing.T) {
	upstreams := []proxy.Upstream{
		{URL: "http://upstream-a"},
		{URL: "http://upstream-b"},
		{URL: "http://upstream-c"},
	}

	balancer, err := proxy.NewLeastConnections(upstreams)
	if err != nil {
		t.Fatalf("create least-connections balancer: %v", err)
	}

	const calls = 300
	errs := make(chan error, calls)
	var wg sync.WaitGroup

	for range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()

			upstream, err := balancer.Acquire()
			if err != nil {
				errs <- err
				return
			}

			errs <- balancer.Release(upstream)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent acquire/release: %v", err)
		}
	}

	for _, upstream := range upstreams {
		err := balancer.Release(upstream)
		if !errors.Is(err, proxy.ErrNoActiveRequests) {
			t.Fatalf("release inactive upstream %q: error = %v, want %v", upstream.URL, err, proxy.ErrNoActiveRequests)
		}
	}
}
