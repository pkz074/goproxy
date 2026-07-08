package tests

import (
	"errors"
	"sync"
	"testing"

	"github.com/pkz074/goproxy/internal/proxy"
)

func TestRoundRobinReturnsUpstreamsInOrderAndWraps(t *testing.T) {
	balancer, err := proxy.NewRoundRobin([]proxy.Upstream{
		{URL: "http://upstream-a"},
		{URL: "http://upstream-b"},
		{URL: "http://upstream-c"},
	})
	if err != nil {
		t.Fatalf("create round robin: %v", err)
	}

	want := []string{
		"http://upstream-a",
		"http://upstream-b",
		"http://upstream-c",
		"http://upstream-a",
	}

	for i, wantURL := range want {
		upstream, err := balancer.Next()
		if err != nil {
			t.Fatalf("Next call %d: %v", i+1, err)
		}

		if upstream.URL != wantURL {
			t.Fatalf("Next call %d returned %q, want %q", i+1, upstream.URL, wantURL)
		}
	}
}

func TestNewRoundRobinRejectsEmptyPool(t *testing.T) {
	balancer, err := proxy.NewRoundRobin(nil)

	if !errors.Is(err, proxy.ErrNoUpstreams) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrNoUpstreams)
	}

	if balancer != nil {
		t.Fatalf("balancer = %#v, want nil", balancer)
	}
}

func TestRoundRobinCopiesUpstreamSlice(t *testing.T) {
	upstreams := []proxy.Upstream{
		{URL: "http://upstream-a"},
	}

	balancer, err := proxy.NewRoundRobin(upstreams)
	if err != nil {
		t.Fatalf("create round robin: %v", err)
	}

	upstreams[0].URL = "http://upstream-b"

	upstream, err := balancer.Next()
	if err != nil {
		t.Fatalf("get next upstream: %v", err)
	}

	if upstream.URL != "http://upstream-a" {
		t.Fatalf("upstream = %q, want %q", upstream.URL, "http://upstream-a")
	}
}

func TestNewRoundRobinRejectsDuplicateUpstream(t *testing.T) {
	balancer, err := proxy.NewRoundRobin([]proxy.Upstream{
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

func TestRoundRobinConcurrentCalls(t *testing.T) {
	balancer, err := proxy.NewRoundRobin([]proxy.Upstream{
		{URL: "http://upstream-a"},
		{URL: "http://upstream-b"},
		{URL: "http://upstream-c"},
	})
	if err != nil {
		t.Fatalf("create round robin: %v", err)
	}

	const calls = 300
	type result struct {
		upstream proxy.Upstream
		err      error
	}

	results := make(chan result, calls)
	var wg sync.WaitGroup

	for range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			upstream, err := balancer.Next()
			results <- result{upstream: upstream, err: err}
		}()
	}

	wg.Wait()
	close(results)

	counts := make(map[string]int)
	for result := range results {
		if result.err != nil {
			t.Fatalf("get next upstream: %v", result.err)
		}
		counts[result.upstream.URL]++
	}

	for _, upstreamURL := range []string{
		"http://upstream-a",
		"http://upstream-b",
		"http://upstream-c",
	} {
		if counts[upstreamURL] != calls/3 {
			t.Fatalf("%s selected %d times, want %d", upstreamURL, counts[upstreamURL], calls/3)
		}
	}
}
