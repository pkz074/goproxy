package proxy

import (
	"sync"
)

type RoundRobin struct {
	mu        sync.Mutex
	upstreams []Upstream
	next      int
}

func NewRoundRobin(upstreams []Upstream) (*RoundRobin, error) {
	if len(upstreams) == 0 {
		return nil, ErrNoUpstreams
	}

	upstreamCopy := append([]Upstream(nil), upstreams...)

	return &RoundRobin{
		upstreams: upstreamCopy,
		next:      0,
	}, nil
}

func (r *RoundRobin) Next() (Upstream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.upstreams) == 0 {
		return Upstream{}, ErrNoUpstreams
	}

	target := r.upstreams[r.next]
	r.next = (r.next + 1) % len(r.upstreams)

	return target, nil
}
