package proxy

import (
	"errors"
	"sync"
)

type BalancingStrategy string

const (
	StrategyRoundRobin         BalancingStrategy = "round_robin"
	StrategyLeastConnections   BalancingStrategy = "least_connections"
	StrategyWeightedRoundRobin BalancingStrategy = "weighted_round_robin"
)

var (
	ErrNoUpstreams       = errors.New("load balancer requires at least one upstream")
	ErrDuplicateUpstream = errors.New("load balancer contains a duplicate upstream")
	ErrUnknownUpstream   = errors.New("upstream is not part of this pool")
	ErrNoActiveRequests  = errors.New("upstream has no active requests")
	ErrInvalidWeight     = errors.New("upstream weight cannot be negative")
	ErrInvalidRoute      = errors.New("route must define exactly one upstream source")
	ErrUnknownStrategy   = errors.New("unknown load balancing strategy")
	ErrContextRequired   = errors.New("health checks require NewRoutedWithContext")
)

type routeBalancer interface {
	Acquire() (Upstream, func(), error)
}

type roundRobinRouteBalancer struct {
	balancer *RoundRobin
}

func (r *roundRobinRouteBalancer) Acquire() (Upstream, func(), error) {
	upstream, err := r.balancer.Next()
	return upstream, func() {}, err
}

type weightedRoundRobinRouteBalancer struct {
	balancer *WeightedRoundRobin
}

func (w *weightedRoundRobinRouteBalancer) Acquire() (Upstream, func(), error) {
	upstream, err := w.balancer.Next()
	return upstream, func() {}, err
}

type leastConnectionsRouteBalancer struct {
	balancer *LeastConnections
}

func (l *leastConnectionsRouteBalancer) Acquire() (Upstream, func(), error) {
	upstream, err := l.balancer.Acquire()
	if err != nil {
		return Upstream{}, func() {}, err
	}

	var once sync.Once
	return upstream, func() {
		once.Do(func() {
			_ = l.balancer.Release(upstream)
		})
	}, nil
}
