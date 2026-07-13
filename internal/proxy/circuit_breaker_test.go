package proxy

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterFailureThreshold(t *testing.T) {
	clock := newFakeClock()
	breaker, err := newCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create circuit breaker: %v", err)
	}

	breaker.RecordFailure()
	if breaker.State() != CircuitClosed {
		t.Fatalf("state = %s, want %s", breaker.State(), CircuitClosed)
	}

	breaker.RecordFailure()
	if breaker.State() != CircuitOpen {
		t.Fatalf("state = %s, want %s", breaker.State(), CircuitOpen)
	}

	if breaker.Allow() {
		t.Fatal("open circuit should reject requests before timeout")
	}
}

func TestCircuitBreakerAllowsHalfOpenProbeAfterTimeout(t *testing.T) {
	clock := newFakeClock()
	breaker, err := newCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create circuit breaker: %v", err)
	}

	breaker.RecordFailure()
	clock.Advance(time.Second)

	if !breaker.Allow() {
		t.Fatal("first half-open probe should be allowed")
	}

	if breaker.Allow() {
		t.Fatal("second half-open probe should be rejected while probe is in flight")
	}

	if breaker.State() != CircuitHalfOpen {
		t.Fatalf("state = %s, want %s", breaker.State(), CircuitHalfOpen)
	}
}

func TestCircuitBreakerClosesAfterSuccessfulHalfOpenProbe(t *testing.T) {
	clock := newFakeClock()
	breaker, err := newCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create circuit breaker: %v", err)
	}

	breaker.RecordFailure()
	clock.Advance(time.Second)

	if !breaker.Allow() {
		t.Fatal("half-open probe should be allowed")
	}

	breaker.RecordSuccess()

	if breaker.State() != CircuitClosed {
		t.Fatalf("state = %s, want %s", breaker.State(), CircuitClosed)
	}

	if !breaker.Allow() {
		t.Fatal("closed circuit should allow requests")
	}
}

func TestCircuitBreakerReopensAfterFailedHalfOpenProbe(t *testing.T) {
	clock := newFakeClock()
	breaker, err := newCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create circuit breaker: %v", err)
	}

	breaker.RecordFailure()
	clock.Advance(time.Second)

	if !breaker.Allow() {
		t.Fatal("half-open probe should be allowed")
	}

	breaker.RecordFailure()

	if breaker.State() != CircuitOpen {
		t.Fatalf("state = %s, want %s", breaker.State(), CircuitOpen)
	}
}

func TestCircuitBreakerIgnoresLateResultsAfterOpening(t *testing.T) {
	clock := newFakeClock()
	breaker, err := newCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  time.Second,
	}, clock.Now)
	if err != nil {
		t.Fatalf("create circuit breaker: %v", err)
	}

	if !breaker.Allow() || !breaker.Allow() {
		t.Fatal("closed circuit should allow concurrent requests")
	}

	breaker.RecordFailure()
	breaker.RecordSuccess()

	if breaker.State() != CircuitOpen {
		t.Fatalf("state = %s, want %s", breaker.State(), CircuitOpen)
	}

	clock.Advance(time.Second)
	if !breaker.Allow() {
		t.Fatal("circuit should allow a half-open probe after timeout")
	}
}

func TestCircuitBreakerRejectsInvalidConfig(t *testing.T) {
	breaker, err := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 0,
		RecoveryTimeout:  time.Second,
	})

	if !errors.Is(err, ErrInvalidFailureThreshold) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidFailureThreshold)
	}

	if breaker != nil {
		t.Fatalf("breaker = %#v, want nil", breaker)
	}

	breaker, err = NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  0,
	})

	if !errors.Is(err, ErrInvalidRecoveryTimeout) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidRecoveryTimeout)
	}

	if breaker != nil {
		t.Fatalf("breaker = %#v, want nil", breaker)
	}
}

func TestCircuitBreakerConcurrentAccess(t *testing.T) {
	breaker, err := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1000,
		RecoveryTimeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("create circuit breaker: %v", err)
	}

	var wg sync.WaitGroup
	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if breaker.Allow() {
				breaker.RecordSuccess()
			}
		}()
	}

	wg.Wait()

	if breaker.State() != CircuitClosed {
		t.Fatalf("state = %s, want %s", breaker.State(), CircuitClosed)
	}
}
