package launch

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"
	StateHalfOpen CircuitState = "HALF_OPEN"
	StateOpen     CircuitState = "OPEN"
)

// CircuitBreaker provides protection against cascading system failures (Task 093).
type CircuitBreaker struct {
	mu           sync.RWMutex
	state        CircuitState
	failureCount int
	threshold    int
	cooldown     time.Duration
	lastFailure  time.Time
}

// NewCircuitBreaker creates a circuit breaker with a given failure threshold and cooldown period.
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 10 * time.Second
	}
	return &CircuitBreaker{
		state:     StateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.cooldown {
		cb.state = StateHalfOpen
	}
	return cb.state
}

// Execute wraps an operation with circuit breaking logic.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	state := cb.State()
	if state == StateOpen {
		return errors.New("circuit breaker is open: request fast-failed")
	}

	err := fn()
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()
		if cb.failureCount >= cb.threshold {
			cb.state = StateOpen
		}
		return err
	}

	// Success
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failureCount = 0
	} else if cb.state == StateClosed && cb.failureCount > 0 {
		cb.failureCount--
	}
	return nil
}

// FaultInjector introduces controlled latency or errors for resilience testing.
type FaultInjector struct {
	mu            sync.RWMutex
	injectedDelay time.Duration
	failRate      float64 // 0.0 to 1.0
	enabled       bool
}

// NewFaultInjector initializes a fault injector.
func NewFaultInjector() *FaultInjector {
	return &FaultInjector{}
}

// Configure sets chaos parameters.
func (fi *FaultInjector) Configure(enabled bool, delay time.Duration, failRate float64) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.enabled = enabled
	fi.injectedDelay = delay
	fi.failRate = failRate
}

// Intercept injects latency or synthetic fault if configured.
func (fi *FaultInjector) Intercept(ctx context.Context, sample float64) error {
	fi.mu.RLock()
	enabled := fi.enabled
	delay := fi.injectedDelay
	failRate := fi.failRate
	fi.mu.RUnlock()

	if !enabled {
		return nil
	}

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if sample < failRate {
		return errors.New("injected chaos fault: synthetic dependency timeout")
	}

	return nil
}

