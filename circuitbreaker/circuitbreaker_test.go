package circuitbreaker

import (
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := New(5, 30*time.Second)
	
	if cb.GetState() != StateClosed {
		t.Error("circuit breaker should start in closed state")
	}
}

func TestCircuitBreaker_OpenAfterThreshold(t *testing.T) {
	cb := New(3, 30*time.Second)
	
	// Record failures up to threshold
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	
	if cb.GetState() != StateOpen {
		t.Error("circuit breaker should be open after reaching threshold")
	}
	
	// Should reject requests when open
	if cb.AllowRequest() {
		t.Error("should not allow requests when circuit is open")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := New(3, 100*time.Millisecond)
	
	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	
	if cb.GetState() != StateOpen {
		t.Error("circuit should be open")
	}
	
	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)
	
	// Should allow request (transitions to half-open)
	if !cb.AllowRequest() {
		t.Error("should allow request after recovery timeout")
	}
	
	if cb.GetState() != StateHalfOpen {
		t.Error("circuit should be in half-open state")
	}
}

func TestCircuitBreaker_CloseOnSuccess(t *testing.T) {
	cb := New(3, 100*time.Millisecond)
	
	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	
	// Wait and allow request
	time.Sleep(150 * time.Millisecond)
	cb.AllowRequest()
	
	// Record success - should close circuit
	cb.RecordSuccess()
	
	if cb.GetState() != StateClosed {
		t.Error("circuit should be closed after success in half-open state")
	}
}

func TestCircuitBreaker_ReopenOnFailure(t *testing.T) {
	cb := New(3, 100*time.Millisecond)
	
	// Open the circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	
	// Wait and allow request
	time.Sleep(150 * time.Millisecond)
	cb.AllowRequest()
	
	// Record failure - should reopen circuit
	cb.RecordFailure()
	
	if cb.GetState() != StateOpen {
		t.Error("circuit should reopen after failure in half-open state")
	}
}

func TestCircuitBreaker_ResetFailureCountOnSuccess(t *testing.T) {
	cb := New(3, 30*time.Second)
	
	// Record 2 failures
	cb.RecordFailure()
	cb.RecordFailure()
	
	if cb.GetFailureCount() != 2 {
		t.Errorf("expected 2 failures, got %d", cb.GetFailureCount())
	}
	
	// Record success - should reset failure count
	cb.RecordSuccess()
	
	if cb.GetFailureCount() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.GetFailureCount())
	}
}

func TestCircuitBreaker_StateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateHalfOpen, "half-open"},
		{StateOpen, "open"},
	}
	
	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}
