package vpn

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// RetryPolicy defines retry behavior with exponential backoff and jitter
type RetryPolicy struct {
	MaxAttempts   int           // Maximum number of attempts (default: 5)
	InitialDelay  time.Duration // Initial delay between retries (default: 1s)
	MaxDelay      time.Duration // Maximum delay cap (default: 30s)
	BackoffFactor float64       // Multiplier for delay increase (default: 2.0)
	JitterFactor  float64       // Random jitter factor 0-1 (default: 0.1)
}

// NewDefaultRetryPolicy creates a RetryPolicy with sensible defaults
func NewDefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   5,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.1,
	}
}

// NewAggressiveRetryPolicy creates a RetryPolicy with more retries and longer delays
func NewAggressiveRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   10,
		InitialDelay:  2 * time.Second,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.15,
	}
}

// NewQuickRetryPolicy creates a RetryPolicy with fewer retries and shorter delays
func NewQuickRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   3,
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.1,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc[T any] func() (T, error)

// RetryableFuncWithContext is a function that can be retried with context
type RetryableFuncWithContext[T any] func(ctx context.Context) (T, error)

// Execute runs a function with retry logic and exponential backoff
func (r *RetryPolicy) Execute(ctx context.Context, fn RetryableFunc[any]) (any, error) {
	return r.executeInternal(ctx, func(_ context.Context) (any, error) {
		return fn()
	})
}

// ExecuteWithContext runs a context-aware function with retry logic
func (r *RetryPolicy) ExecuteWithContext(ctx context.Context, fn RetryableFuncWithContext[any]) (any, error) {
	return r.executeInternal(ctx, fn)
}

// ExecuteNoResult runs a function that returns only an error with retry logic
func (r *RetryPolicy) ExecuteNoResult(ctx context.Context, fn func() error) error {
	_, err := r.executeInternal(ctx, func(_ context.Context) (any, error) {
		return nil, fn()
	})
	return err
}

// executeInternal is the core retry logic
func (r *RetryPolicy) executeInternal(ctx context.Context, fn RetryableFuncWithContext[any]) (any, error) {
	var lastErr error
	delay := r.InitialDelay

	for attempt := 1; attempt <= r.MaxAttempts; attempt++ {
		// Check context before attempting
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled before attempt %d: %w", attempt, ctx.Err())
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't wait after the last attempt
		if attempt == r.MaxAttempts {
			break
		}

		// Check context after failed attempt
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled after attempt %d: %w", attempt, ctx.Err())
		}

		// Calculate delay with jitter
		jitterRange := float64(delay) * r.JitterFactor
		jitter := time.Duration(jitterRange * (rand.Float64()*2 - 1))
		actualDelay := delay + jitter

		// Wait with context awareness
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
		case <-time.After(actualDelay):
		}

		// Increase delay for next attempt (exponential backoff)
		delay = time.Duration(float64(delay) * r.BackoffFactor)
		if delay > r.MaxDelay {
			delay = r.MaxDelay
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", r.MaxAttempts, lastErr)
}

// RetryResult contains information about a retry operation
type RetryResult struct {
	Attempts  int
	LastError error
	Success   bool
	Duration  time.Duration
}

// ExecuteWithResult runs a function with retry and returns detailed result info
func (r *RetryPolicy) ExecuteWithResult(ctx context.Context, fn RetryableFunc[any]) (*RetryResult, any, error) {
	startTime := time.Now()
	result := &RetryResult{}
	delay := r.InitialDelay

	for attempt := 1; attempt <= r.MaxAttempts; attempt++ {
		result.Attempts = attempt

		if ctx.Err() != nil {
			result.LastError = ctx.Err()
			result.Duration = time.Since(startTime)
			return result, nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		value, err := fn()
		if err == nil {
			result.Success = true
			result.Duration = time.Since(startTime)
			return result, value, nil
		}

		result.LastError = err

		if attempt == r.MaxAttempts {
			break
		}

		// Wait with jitter
		jitterRange := float64(delay) * r.JitterFactor
		jitter := time.Duration(jitterRange * (rand.Float64()*2 - 1))
		actualDelay := delay + jitter

		select {
		case <-ctx.Done():
			result.Duration = time.Since(startTime)
			return result, nil, fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
		case <-time.After(actualDelay):
		}

		delay = time.Duration(float64(delay) * r.BackoffFactor)
		if delay > r.MaxDelay {
			delay = r.MaxDelay
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil, fmt.Errorf("max retries (%d) exceeded: %w", r.MaxAttempts, result.LastError)
}

// IsRetryable checks if an error is retryable (can be extended with custom logic)
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// By default, all errors are retryable
	// This can be extended to check for specific error types
	return true
}

// WithMaxAttempts returns a copy of the policy with a different max attempts
func (r *RetryPolicy) WithMaxAttempts(attempts int) *RetryPolicy {
	copy := *r
	copy.MaxAttempts = attempts
	return &copy
}

// WithInitialDelay returns a copy of the policy with a different initial delay
func (r *RetryPolicy) WithInitialDelay(delay time.Duration) *RetryPolicy {
	copy := *r
	copy.InitialDelay = delay
	return &copy
}

// WithMaxDelay returns a copy of the policy with a different max delay
func (r *RetryPolicy) WithMaxDelay(delay time.Duration) *RetryPolicy {
	copy := *r
	copy.MaxDelay = delay
	return &copy
}
