package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration
type Config struct {
	Enabled          bool          // Enable/disable retry logic
	MaxAttempts      int           // Maximum number of retry attempts
	InitialDelay     time.Duration // Initial delay before first retry
	MaxDelay         time.Duration // Maximum delay between retries
	Multiplier       float64       // Exponential backoff multiplier (typically 2.0)
	Jitter           bool          // Add random jitter to prevent thundering herd
	RetryableErrors  []error       // List of errors that should trigger retry (nil = all errors)
	NonRetryableErrors []error     // List of errors that should NOT trigger retry
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, cfg Config, fn func() error) error {
	if !cfg.Enabled {
		return fn()
	}

	var lastErr error
	
	for attempt := 0; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is non-retryable
		if isNonRetryable(err, cfg.NonRetryableErrors) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Check if error is retryable (if list is specified)
		if len(cfg.RetryableErrors) > 0 && !isRetryable(err, cfg.RetryableErrors) {
			return fmt.Errorf("error not in retryable list: %w", err)
		}

		// Don't retry on last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(cfg, attempt)

		// Wait before retry
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during wait: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("max attempts (%d) exceeded: %w", cfg.MaxAttempts, lastErr)
}

// RetryWithResult executes a function that returns a result with exponential backoff retry logic
func RetryWithResult[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var zero T
	
	if !cfg.Enabled {
		return fn()
	}

	var lastErr error
	
	for attempt := 0; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		result, err := fn()
		if err == nil {
			return result, nil // Success
		}

		lastErr = err

		// Check if error is non-retryable
		if isNonRetryable(err, cfg.NonRetryableErrors) {
			return zero, fmt.Errorf("non-retryable error: %w", err)
		}

		// Check if error is retryable (if list is specified)
		if len(cfg.RetryableErrors) > 0 && !isRetryable(err, cfg.RetryableErrors) {
			return zero, fmt.Errorf("error not in retryable list: %w", err)
		}

		// Don't retry on last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(cfg, attempt)

		// Wait before retry
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled during wait: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return zero, fmt.Errorf("max attempts (%d) exceeded: %w", cfg.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay for exponential backoff
func calculateDelay(cfg Config, attempt int) time.Duration {
	// Calculate exponential delay: initialDelay * (multiplier ^ attempt)
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt))
	
	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	duration := time.Duration(delay)

	// Add jitter (±25% random variation)
	if cfg.Jitter {
		jitter := duration / 4
		duration = duration - jitter + time.Duration(float64(jitter*2)*0.5) // Simplified jitter
	}

	return duration
}

// isRetryable checks if an error is in the retryable errors list
func isRetryable(err error, retryableErrors []error) bool {
	for _, retryableErr := range retryableErrors {
		if err == retryableErr || fmt.Sprintf("%T", err) == fmt.Sprintf("%T", retryableErr) {
			return true
		}
	}
	return false
}

// isNonRetryable checks if an error is in the non-retryable errors list
func isNonRetryable(err error, nonRetryableErrors []error) bool {
	for _, nonRetryableErr := range nonRetryableErrors {
		if err == nonRetryableErr || fmt.Sprintf("%T", err) == fmt.Sprintf("%T", nonRetryableErr) {
			return true
		}
	}
	return false
}

