package workers

import (
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	if cb.GetState() != StateClosed {
		t.Errorf("Initial state should be CLOSED, got %s", cb.GetState())
	}

	if !cb.AllowRequest() {
		t.Error("CircuitBreaker should allow requests when closed")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.GetState() != StateClosed {
		t.Error("Should still be closed after 2 failures (threshold is 3)")
	}

	cb.RecordFailure()
	if cb.GetState() != StateOpen {
		t.Errorf("Should be OPEN after 3 failures, got %s", cb.GetState())
	}

	if cb.AllowRequest() {
		t.Error("CircuitBreaker should NOT allow requests when open")
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	// Failures should be reset after success
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.GetState() != StateClosed {
		t.Error("Should still be closed — success should have reset failure count")
	}
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond) // very short timeout for test

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.GetState() != StateOpen {
		t.Error("Should be OPEN after threshold")
	}

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	cb.TryTransitionToHalfOpen()
	if cb.GetState() != StateHalfOpen {
		t.Errorf("Should be HALF_OPEN after recovery timeout, got %s", cb.GetState())
	}

	if !cb.AllowRequest() {
		t.Error("CircuitBreaker should allow requests when half-open")
	}

	// Success in half-open should close
	cb.RecordSuccess()
	if cb.GetState() != StateClosed {
		t.Errorf("Should be CLOSED after success in half-open, got %s", cb.GetState())
	}
}
