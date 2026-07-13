package proxy

import (
	"errors"
	"sync"
	"time"
)

type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold int
	RecoveryTimeout  time.Duration
}

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

var (
	ErrInvalidFailureThreshold = errors.New("circuit breaker failure threshold must be greater than zero")
	ErrInvalidRecoveryTimeout  = errors.New("circuit breaker recovery timeout must be greater than zero")
)

type CircuitBreaker struct {
	mu            sync.Mutex
	config        CircuitBreakerConfig
	state         CircuitState
	failures      int
	openedAt      time.Time
	probeInFlight bool
	now           func() time.Time
}

func NewCircuitBreaker(config CircuitBreakerConfig) (*CircuitBreaker, error) {
	return newCircuitBreaker(config, time.Now)
}

func (c *CircuitBreaker) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if c.now().Sub(c.openedAt) < c.config.RecoveryTimeout {
			return false
		}

		c.state = CircuitHalfOpen
		c.probeInFlight = true
		return true
	case CircuitHalfOpen:
		if c.probeInFlight {
			return false
		}

		c.probeInFlight = true
		return true
	default:
		return false
	}
}

func (c *CircuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitClosed:
		c.failures = 0
	case CircuitHalfOpen:
		c.state = CircuitClosed
		c.failures = 0
		c.probeInFlight = false
	}
}

func (c *CircuitBreaker) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitClosed:
		c.failures++
		if c.failures >= c.config.FailureThreshold {
			c.open()
		}
	case CircuitHalfOpen:
		c.open()
	case CircuitOpen:
		return
	}

	c.probeInFlight = false
}

func (c *CircuitBreaker) State() CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.state
}

func newCircuitBreaker(config CircuitBreakerConfig, now func() time.Time) (*CircuitBreaker, error) {
	if config.FailureThreshold <= 0 {
		return nil, ErrInvalidFailureThreshold
	}

	if config.RecoveryTimeout <= 0 {
		return nil, ErrInvalidRecoveryTimeout
	}

	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
		now:    now,
	}, nil
}

func (c *CircuitBreaker) open() {
	c.state = CircuitOpen
	c.openedAt = c.now()
	c.probeInFlight = false
}
