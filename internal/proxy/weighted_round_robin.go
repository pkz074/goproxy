package proxy

import "sync"

type WeightedUpstream struct {
	Upstream Upstream
	Weight   int
}

type WeightedRoundRobin struct {
	mu        sync.Mutex
	upstreams []WeightedUpstream
	current   []int
	total     int
}

func NewWeightedRoundRobin(upstreams []WeightedUpstream) (*WeightedRoundRobin, error) {
	if len(upstreams) == 0 {
		return nil, ErrNoUpstreams
	}

	upstreamCopy := make([]WeightedUpstream, len(upstreams))
	current := make([]int, len(upstreams))
	seen := make(map[string]struct{}, len(upstreams))
	total := 0

	for i, upstream := range upstreams {
		if _, ok := seen[upstream.Upstream.URL]; ok {
			return nil, ErrDuplicateUpstream
		}

		if upstream.Weight < 0 {
			return nil, ErrInvalidWeight
		}

		if upstream.Weight == 0 {
			upstream.Weight = 1
		}

		seen[upstream.Upstream.URL] = struct{}{}
		upstreamCopy[i] = upstream
		total += upstream.Weight
	}

	return &WeightedRoundRobin{
		upstreams: upstreamCopy,
		current:   current,
		total:     total,
	}, nil
}

func (w *WeightedRoundRobin) Next() (Upstream, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.upstreams) == 0 {
		return Upstream{}, ErrNoUpstreams
	}

	selected := 0
	for i, upstream := range w.upstreams {
		w.current[i] += upstream.Weight
		if w.current[i] > w.current[selected] {
			selected = i
		}
	}

	w.current[selected] -= w.total

	return w.upstreams[selected].Upstream, nil
}
