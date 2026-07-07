package tests

import (
	"errors"
	"sync"
	"testing"

	"github.com/pkz074/goproxy/internal/proxy"
)

func TestWeightedRoundRobinEqualWeightsBehavesLikeRoundRobin(t *testing.T) {
	balancer, err := proxy.NewWeightedRoundRobin([]proxy.WeightedUpstream{
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: 1},
		{Upstream: proxy.Upstream{URL: "http://upstream-b"}, Weight: 1},
		{Upstream: proxy.Upstream{URL: "http://upstream-c"}, Weight: 1},
	})
	if err != nil {
		t.Fatalf("create weighted round robin: %v", err)
	}

	want := []string{
		"http://upstream-a",
		"http://upstream-b",
		"http://upstream-c",
		"http://upstream-a",
	}

	assertWeightedSequence(t, balancer, want)
}

func TestWeightedRoundRobinHonorsWeights(t *testing.T) {
	balancer, err := proxy.NewWeightedRoundRobin([]proxy.WeightedUpstream{
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: 3},
		{Upstream: proxy.Upstream{URL: "http://upstream-b"}, Weight: 1},
	})
	if err != nil {
		t.Fatalf("create weighted round robin: %v", err)
	}

	want := []string{
		"http://upstream-a",
		"http://upstream-a",
		"http://upstream-b",
		"http://upstream-a",
		"http://upstream-a",
		"http://upstream-a",
		"http://upstream-b",
		"http://upstream-a",
	}

	assertWeightedSequence(t, balancer, want)
}

func TestWeightedRoundRobinDefaultsZeroWeightToOne(t *testing.T) {
	balancer, err := proxy.NewWeightedRoundRobin([]proxy.WeightedUpstream{
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: 0},
		{Upstream: proxy.Upstream{URL: "http://upstream-b"}, Weight: 0},
	})
	if err != nil {
		t.Fatalf("create weighted round robin: %v", err)
	}

	want := []string{
		"http://upstream-a",
		"http://upstream-b",
		"http://upstream-a",
		"http://upstream-b",
	}

	assertWeightedSequence(t, balancer, want)
}

func TestNewWeightedRoundRobinRejectsEmptyPool(t *testing.T) {
	balancer, err := proxy.NewWeightedRoundRobin(nil)

	if !errors.Is(err, proxy.ErrNoUpstreams) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrNoUpstreams)
	}

	if balancer != nil {
		t.Fatalf("balancer = %#v, want nil", balancer)
	}
}

func TestNewWeightedRoundRobinRejectsNegativeWeight(t *testing.T) {
	balancer, err := proxy.NewWeightedRoundRobin([]proxy.WeightedUpstream{
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: -1},
	})

	if !errors.Is(err, proxy.ErrInvalidWeight) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrInvalidWeight)
	}

	if balancer != nil {
		t.Fatalf("balancer = %#v, want nil", balancer)
	}
}

func TestNewWeightedRoundRobinRejectsDuplicateUpstream(t *testing.T) {
	balancer, err := proxy.NewWeightedRoundRobin([]proxy.WeightedUpstream{
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: 1},
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: 2},
	})

	if !errors.Is(err, proxy.ErrDuplicateUpstream) {
		t.Fatalf("error = %v, want %v", err, proxy.ErrDuplicateUpstream)
	}

	if balancer != nil {
		t.Fatalf("balancer = %#v, want nil", balancer)
	}
}

func TestWeightedRoundRobinCopiesUpstreamSlice(t *testing.T) {
	upstreams := []proxy.WeightedUpstream{
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: 1},
	}

	balancer, err := proxy.NewWeightedRoundRobin(upstreams)
	if err != nil {
		t.Fatalf("create weighted round robin: %v", err)
	}

	upstreams[0].Upstream.URL = "http://upstream-b"
	upstreams[0].Weight = 100

	upstream, err := balancer.Next()
	if err != nil {
		t.Fatalf("get next upstream: %v", err)
	}

	if upstream.URL != "http://upstream-a" {
		t.Fatalf("upstream = %q, want %q", upstream.URL, "http://upstream-a")
	}
}

func TestWeightedRoundRobinConcurrentCalls(t *testing.T) {
	balancer, err := proxy.NewWeightedRoundRobin([]proxy.WeightedUpstream{
		{Upstream: proxy.Upstream{URL: "http://upstream-a"}, Weight: 3},
		{Upstream: proxy.Upstream{URL: "http://upstream-b"}, Weight: 1},
	})
	if err != nil {
		t.Fatalf("create weighted round robin: %v", err)
	}

	const calls = 400
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

	if counts["http://upstream-a"] != 300 {
		t.Fatalf("upstream-a selected %d times, want 300", counts["http://upstream-a"])
	}

	if counts["http://upstream-b"] != 100 {
		t.Fatalf("upstream-b selected %d times, want 100", counts["http://upstream-b"])
	}
}

func assertWeightedSequence(t *testing.T, balancer *proxy.WeightedRoundRobin, want []string) {
	t.Helper()

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
