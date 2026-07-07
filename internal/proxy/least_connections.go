package proxy

import "sync"

type LeastConnections struct {
	mu        sync.Mutex
	upstreams []Upstream
	active    []int
}

func NewLeastConnections(upstreams []Upstream) (*LeastConnections, error) {
	if len(upstreams) == 0 {
		return nil, ErrNoUpstreams
	}

	seen := make(map[string]struct{}, len(upstreams))
	for _, upstream := range upstreams {
		if _, ok := seen[upstream.URL]; ok {
			return nil, ErrDuplicateUpstream
		}

		seen[upstream.URL] = struct{}{}
	}

	upstreamCopy := append([]Upstream(nil), upstreams...)

	return &LeastConnections{
		upstreams: upstreamCopy,
		active:    make([]int, len(upstreamCopy)),
	}, nil
}

func (l *LeastConnections) Acquire() (Upstream, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.upstreams) == 0 {
		return Upstream{}, ErrNoUpstreams
	}

	selected := 0
	for i := 1; i < len(l.active); i++ {
		if l.active[i] < l.active[selected] {
			selected = i
		}
	}

	l.active[selected]++

	return l.upstreams[selected], nil
}

func (l *LeastConnections) Release(upstream Upstream) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, current := range l.upstreams {
		if current.URL != upstream.URL {
			continue
		}

		if l.active[i] == 0 {
			return ErrNoActiveRequests
		}

		l.active[i]--
		return nil
	}

	return ErrUnknownUpstream
}
