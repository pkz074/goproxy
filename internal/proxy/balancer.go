package proxy

import "errors"

var (
	ErrNoUpstreams       = errors.New("load balancer requires at least one upstream")
	ErrDuplicateUpstream = errors.New("load balancer contains a duplicate upstream")
	ErrUnknownUpstream   = errors.New("upstream is not part of this pool")
	ErrNoActiveRequests  = errors.New("upstream has no active requests")
	ErrInvalidWeight     = errors.New("upstream weight cannot be negative")
)
