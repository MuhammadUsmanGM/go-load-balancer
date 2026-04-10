package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateHalfOpen               // Testing if backend recovered
	StateOpen                   // Failing, rejecting requests
)

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu sync.RWMutex

	// Configuration
	failureThreshold int64         // failures before opening circuit
	recoveryTimeout  time.Duration // time before trying half-open

	// State
	state          State
	failureCount   int64
	successCount   int64
	lastFailure    time.Time
	lastStateChange time.Time
}

// New creates a new circuit breaker.
func New(failureThreshold int64, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:           StateClosed,
		failureThreshold: failureThreshold,
		recoveryTimeout: recoveryTimeout,
		lastStateChange: time.Now(),
	}
}

// AllowRequest checks if a request should be allowed.
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.RLock()
	currentState := cb.state
	cb.mu.RUnlock()

	switch currentState {
	case StateClosed:
		return true
	case StateOpen:
		// Check if recovery timeout has elapsed
		if time.Since(cb.lastFailure) > cb.recoveryTimeout {
			cb.mu.Lock()
			if cb.state == StateOpen {
				cb.state = StateHalfOpen
				cb.lastStateChange = time.Now()
			}
			cb.mu.Unlock()
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++

	if cb.state == StateHalfOpen {
		// Reset to closed state
		cb.state = StateClosed
		cb.failureCount = 0
		cb.successCount = 0
		cb.lastStateChange = time.Now()
	} else if cb.state == StateClosed {
		// Reset failure count on success
		cb.failureCount = 0
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.state == StateHalfOpen {
		// Back to open state
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	} else if cb.state == StateClosed && cb.failureCount >= cb.failureThreshold {
		// Open the circuit
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	}
}

// GetState returns the current state.
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count.
func (cb *CircuitBreaker) GetFailureCount() int64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// GetStateName returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}
