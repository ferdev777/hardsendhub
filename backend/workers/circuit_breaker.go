package workers

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker.
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "CLOSED"
	StateOpen     CircuitBreakerState = "OPEN"
	StateHalfOpen CircuitBreakerState = "HALF_OPEN"

	// Bounce rate monitor constants (Capa 4 — Anti-Bounce Engine)
	bounceWindowSize      = 1000  // Sliding window: evaluate bounce rate every N sends
	bounceRateThreshold   = 0.018 // 1.8% hard bounce rate triggers pause
	minBounceCountTrigger = 25    // Also trigger if >= 25 hard bounces in the window
)

// BounceRateMonitor tracks hard bounces in a sliding window of sends.
// It is separate from the technical circuit breaker (network errors) so that
// reputation bounces and transient API errors are not confused.
type BounceRateMonitor struct {
	mu                   sync.Mutex
	windowSent           int64
	windowBounced        int64
	totalSent            int64
	totalBounced         int64
	alertTriggered       int32 // atomic: 1 = bounce alert active
	onBounceAlert        func(bounceRate float64, windowBounced, windowSent int)
}

// NewBounceRateMonitor creates a new bounce rate monitor with an alert callback.
func NewBounceRateMonitor(onAlert func(bounceRate float64, windowBounced, windowSent int)) *BounceRateMonitor {
	return &BounceRateMonitor{onBounceAlert: onAlert}
}

// RecordSent increments the sent counter in the sliding window.
// When the window is full (bounceWindowSize), it evaluates the bounce rate
// and resets the window for the next batch.
func (m *BounceRateMonitor) RecordSent() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.windowSent++
	m.totalSent++

	// Evaluate at end of each window
	if m.windowSent >= bounceWindowSize {
		m.evaluateWindow()
	}
}

// RecordHardBounce increments the bounce counter.
// A hard bounce is a permanent delivery failure (mailbox doesn't exist, domain expired).
func (m *BounceRateMonitor) RecordHardBounce() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.windowBounced++
	m.totalBounced++

	// Early trigger: don't wait for end of window if absolute count is already high
	if m.windowBounced >= minBounceCountTrigger && atomic.LoadInt32(&m.alertTriggered) == 0 {
		rate := float64(m.windowBounced) / float64(m.windowSent+1)
		if rate >= bounceRateThreshold {
			m.triggerAlert(rate)
		}
	}
}

// evaluateWindow checks the bounce rate at the end of a window and resets counters.
// Called with m.mu held.
func (m *BounceRateMonitor) evaluateWindow() {
	if m.windowSent == 0 {
		return
	}
	rate := float64(m.windowBounced) / float64(m.windowSent)
	log.Printf("[BounceMonitor] Window complete: %d sent, %d bounced, rate=%.2f%%",
		m.windowSent, m.windowBounced, rate*100)

	if rate >= bounceRateThreshold && atomic.LoadInt32(&m.alertTriggered) == 0 {
		m.triggerAlert(rate)
	}

	// Reset window for next batch
	m.windowSent = 0
	m.windowBounced = 0
}

// triggerAlert fires the bounce alert callback and marks the monitor as alerted.
// Called with m.mu held.
func (m *BounceRateMonitor) triggerAlert(rate float64) {
	atomic.StoreInt32(&m.alertTriggered, 1)
	log.Printf("[BounceMonitor] ALERT: bounce rate %.2f%% exceeded threshold %.2f%% — pausing campaign",
		rate*100, bounceRateThreshold*100)
	if m.onBounceAlert != nil {
		go m.onBounceAlert(rate, int(m.windowBounced), int(m.windowSent))
	}
}

// IsAlerted returns true if the bounce rate threshold was exceeded.
func (m *BounceRateMonitor) IsAlerted() bool {
	return atomic.LoadInt32(&m.alertTriggered) == 1
}

// ResetAlert clears the bounce alert (operator manually resumes campaign).
func (m *BounceRateMonitor) ResetAlert() {
	atomic.StoreInt32(&m.alertTriggered, 0)
	m.mu.Lock()
	m.windowSent = 0
	m.windowBounced = 0
	m.mu.Unlock()
	log.Println("[BounceMonitor] Alert cleared by operator. Resuming campaign.")
}

// GetStats returns current window and total stats for the WebSocket metrics broadcast.
func (m *BounceRateMonitor) GetStats() (windowSent, windowBounced, totalSent, totalBounced int, rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws, wb, ts, tb := m.windowSent, m.windowBounced, m.totalSent, m.totalBounced
	r := 0.0
	if ts > 0 {
		r = float64(tb) / float64(ts) * 100
	}
	return int(ws), int(wb), int(ts), int(tb), r
}

// --- Original CircuitBreaker (technical errors / API failures) ---

// CircuitBreaker implements the circuit breaker pattern for API errors (network, 429, 5xx).
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
		if time.Since(cb.openedAt) >= cb.recoveryTimeout {
			return true
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
		log.Println("[CircuitBreaker] Transitioned to CLOSED - provider is healthy")
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
		log.Println("[CircuitBreaker] Transitioned to HALF_OPEN - testing provider connection")
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
