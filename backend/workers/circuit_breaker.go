package workers

import (
	"log"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker.
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "CLOSED"
	StateOpen     CircuitBreakerState = "OPEN"
	StateHalfOpen CircuitBreakerState = "HALF_OPEN"
)

// CircuitBreaker implements the circuit breaker pattern for AWS SES.
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitBreakerState
	consecutiveFails int
	failureThreshold int
	recoveryTimeout  time.Duration
	lastFailureTime  time.Time
	openedAt         time.Time
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// AllowRequest checks if a request is allowed through the circuit breaker.
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if recovery timeout has elapsed
		if time.Since(cb.openedAt) >= cb.recoveryTimeout {
			return true // Will transition to half-open on next call
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails = 0
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		log.Println("[CircuitBreaker] Transitioned to CLOSED - SES is healthy")
	}
}

// RecordFailure records a failed operation and potentially opens the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails++
	cb.lastFailureTime = time.Now()

	if cb.consecutiveFails >= cb.failureThreshold {
		cb.state = StateOpen
		cb.openedAt = time.Now()
		log.Printf("[CircuitBreaker] Transitioned to OPEN after %d consecutive failures. Pausing for %v",
			cb.consecutiveFails, cb.recoveryTimeout)
	}
}

// TryTransitionToHalfOpen attempts to transition from OPEN to HALF_OPEN.
func (cb *CircuitBreaker) TryTransitionToHalfOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen && time.Since(cb.openedAt) >= cb.recoveryTimeout {
		cb.state = StateHalfOpen
		cb.consecutiveFails = 0
		log.Println("[CircuitBreaker] Transitioned to HALF_OPEN - testing SES connection")
		return true
	}
	return false
}

// GetState returns the current state of the circuit breaker.
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetTimeUntilRecovery returns the remaining time until recovery when in OPEN state.
func (cb *CircuitBreaker) GetTimeUntilRecovery() time.Duration {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state != StateOpen {
		return 0
	}

	elapsed := time.Since(cb.openedAt)
	if elapsed >= cb.recoveryTimeout {
		return 0
	}
	return cb.recoveryTimeout - elapsed
}
