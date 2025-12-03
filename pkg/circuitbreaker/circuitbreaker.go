package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota // Normal operation, requests pass through
	StateOpen                // Circuit is open, requests fail immediately
	StateHalfOpen            // Testing if service recovered, limited requests allowed
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config holds circuit breaker configuration
type Config struct {
	FailureThreshold    int           // Number of failures before opening circuit
	SuccessThreshold    int           // Number of successes in half-open state to close circuit
	Timeout             time.Duration // Time to wait before transitioning from open to half-open
	MaxRequestsHalfOpen int           // Max requests allowed in half-open state
}

// DefaultConfig returns a default circuit breaker configuration
func DefaultConfig() Config {
	return Config{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		MaxRequestsHalfOpen: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config Config

	mu                sync.RWMutex
	state             State
	failureCount      int
	successCount      int
	halfOpenRequests  int
	lastFailureTime   time.Time
	stateChangeTime   time.Time

	onStateChange func(from, to State)
}

// New creates a new circuit breaker with the given configuration
func New(config Config) *CircuitBreaker {
	return &CircuitBreaker{
		config:        config,
		state:         StateClosed,
		stateChangeTime: time.Now(),
	}
}

// OnStateChange sets a callback function that is called when the circuit breaker state changes
func (cb *CircuitBreaker) OnStateChange(fn func(from, to State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Execute executes a function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if request should be allowed
	if !cb.allowRequest() {
		return fmt.Errorf("circuit breaker is %s, request rejected", cb.getState())
	}

	// Execute the function
	err := fn()

	// Record the result
	if err != nil {
		cb.onFailure()
		return fmt.Errorf("circuit breaker execution failed: %w", err)
	}

	cb.onSuccess()
	return nil
}

// ExecuteWithResult executes a function that returns a result through the circuit breaker
// Uses interface{} for compatibility with Go versions before 1.18
func (cb *CircuitBreaker) ExecuteWithResult(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	// Check if request should be allowed
	if !cb.allowRequest() {
		return nil, fmt.Errorf("circuit breaker is %s, request rejected", cb.getState())
	}

	// Execute the function
	result, err := fn()

	// Record the result
	if err != nil {
		cb.onFailure()
		return nil, fmt.Errorf("circuit breaker execution failed: %w", err)
	}

	cb.onSuccess()
	return result, nil
}

// allowRequest checks if a request should be allowed based on current state
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	// Check if we should transition from open to half-open
	if cb.state == StateOpen {
		if now.Sub(cb.stateChangeTime) >= cb.config.Timeout {
			cb.transitionTo(StateHalfOpen)
			return true
		}
		return false
	}

	// Check half-open state limits
	if cb.state == StateHalfOpen {
		if cb.halfOpenRequests >= cb.config.MaxRequestsHalfOpen {
			return false
		}
		cb.halfOpenRequests++
		return true
	}

	// Closed state - allow all requests
	return true
}

// onFailure records a failure and updates circuit breaker state
func (cb *CircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	// Reset success count on failure
	cb.successCount = 0

	// Transition to open if threshold exceeded
	if cb.state == StateClosed && cb.failureCount >= cb.config.FailureThreshold {
		cb.transitionTo(StateOpen)
	} else if cb.state == StateHalfOpen {
		// Any failure in half-open state goes back to open
		cb.transitionTo(StateOpen)
	}
}

// onSuccess records a success and updates circuit breaker state
func (cb *CircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	cb.failureCount = 0 // Reset failure count on success

	// Transition from half-open to closed if threshold met
	if cb.state == StateHalfOpen && cb.successCount >= cb.config.SuccessThreshold {
		cb.transitionTo(StateClosed)
		cb.halfOpenRequests = 0
	}
}

// transitionTo transitions the circuit breaker to a new state
func (cb *CircuitBreaker) transitionTo(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.stateChangeTime = time.Now()

	// Reset counters on state change
	if newState == StateClosed {
		cb.failureCount = 0
		cb.successCount = 0
		cb.halfOpenRequests = 0
	} else if newState == StateHalfOpen {
		cb.failureCount = 0
		cb.successCount = 0
		cb.halfOpenRequests = 0
	}

	// Call state change callback
	if cb.onStateChange != nil {
		go cb.onStateChange(oldState, newState)
	}
}

// getState returns the current state (thread-safe)
func (cb *CircuitBreaker) getState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() State {
	return cb.getState()
}

// GetStats returns current circuit breaker statistics
func (cb *CircuitBreaker) GetStats() Stats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return Stats{
		State:           cb.state,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		HalfOpenRequests: cb.halfOpenRequests,
		LastFailureTime: cb.lastFailureTime,
		StateChangeTime: cb.stateChangeTime,
	}
}

// Stats holds circuit breaker statistics
type Stats struct {
	State           State
	FailureCount    int
	SuccessCount    int
	HalfOpenRequests int
	LastFailureTime time.Time
	StateChangeTime time.Time
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(StateClosed)
}

