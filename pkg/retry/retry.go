// Package retry provides retry logic with exponential backoff for transient failures
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryableError wraps an error to indicate it can be retried
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryableErr *RetryableError
	return errors.As(err, &retryableErr)
}

// NewRetryableError wraps an error as retryable
func NewRetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// Config holds retry configuration
type Config struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries)
	MaxRetries int

	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases after each retry
	Multiplier float64

	// Jitter adds randomness to delays to prevent thundering herd
	Jitter bool

	// JitterFactor is the maximum jitter as a fraction of delay (0.0 to 1.0)
	JitterFactor float64

	// RetryIf is an optional function to determine if an error should be retried
	// If nil, all errors are retried (up to MaxRetries)
	RetryIf func(error) bool

	// OnRetry is called before each retry attempt
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		JitterFactor: 0.3,
	}
}

// AggressiveConfig returns a Config for critical operations
func AggressiveConfig() Config {
	return Config{
		MaxRetries:   5,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     1 * time.Minute,
		Multiplier:   2.0,
		Jitter:       true,
		JitterFactor: 0.2,
	}
}

// QuickConfig returns a Config for fast operations
func QuickConfig() Config {
	return Config{
		MaxRetries:   2,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		JitterFactor: 0.1,
	}
}

// Retrier handles retry logic
type Retrier struct {
	config Config
	rng    *rand.Rand
}

// New creates a new Retrier with the given config
func New(config Config) *Retrier {
	return &Retrier{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Do executes the function with retry logic
func (r *Retrier) Do(fn func() error) error {
	return r.DoWithContext(context.Background(), fn)
}

// DoWithContext executes the function with retry logic and context
func (r *Retrier) DoWithContext(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check context before attempting
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("context cancelled after %d attempts: %w (last error: %v)", attempt, ctx.Err(), lastErr)
			}
			return ctx.Err()
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !r.shouldRetry(err) {
			return err
		}

		// Check if we've exhausted retries
		if attempt >= r.config.MaxRetries {
			break
		}

		// Calculate delay
		delay := r.calculateDelay(attempt)

		// Call OnRetry callback if set
		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt+1, err, delay)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry wait: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", r.config.MaxRetries, lastErr)
}

// DoWithData executes a function that returns data with retry logic
func DoWithData[T any](r *Retrier, fn func() (T, error)) (T, error) {
	return DoWithDataContext(context.Background(), r, fn)
}

// DoWithDataContext executes a function that returns data with retry logic and context
func DoWithDataContext[T any](ctx context.Context, r *Retrier, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return result, fmt.Errorf("context cancelled after %d attempts: %w (last error: %v)", attempt, ctx.Err(), lastErr)
			}
			return result, ctx.Err()
		default:
		}

		var err error
		result, err = fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !r.shouldRetry(err) {
			return result, err
		}

		if attempt >= r.config.MaxRetries {
			break
		}

		delay := r.calculateDelay(attempt)

		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt+1, err, delay)
		}

		select {
		case <-ctx.Done():
			return result, fmt.Errorf("context cancelled during retry wait: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(delay):
		}
	}

	return result, fmt.Errorf("max retries (%d) exceeded: %w", r.config.MaxRetries, lastErr)
}

func (r *Retrier) shouldRetry(err error) bool {
	if r.config.RetryIf != nil {
		return r.config.RetryIf(err)
	}
	// By default, retry all errors
	return true
}

func (r *Retrier) calculateDelay(attempt int) time.Duration {
	// Calculate base delay with exponential backoff
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt))

	// Apply max delay cap
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Apply jitter if enabled
	if r.config.Jitter && r.config.JitterFactor > 0 {
		jitter := delay * r.config.JitterFactor * (r.rng.Float64()*2 - 1)
		delay += jitter
	}

	// Ensure delay is not negative
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// Convenience functions for common use cases

// Do executes a function with default retry config
func Do(fn func() error) error {
	return New(DefaultConfig()).Do(fn)
}

// DoWithContext executes a function with default retry config and context
func DoWithContext(ctx context.Context, fn func() error) error {
	return New(DefaultConfig()).DoWithContext(ctx, fn)
}

// WithRetries executes a function with a specific number of retries
func WithRetries(maxRetries int, fn func() error) error {
	config := DefaultConfig()
	config.MaxRetries = maxRetries
	return New(config).Do(fn)
}

// WithConfig executes a function with a custom config
func WithConfig(config Config, fn func() error) error {
	return New(config).Do(fn)
}

// Backoff represents a backoff strategy
type Backoff struct {
	attempt      int
	initialDelay time.Duration
	maxDelay     time.Duration
	multiplier   float64
	jitter       bool
	rng          *rand.Rand
}

// NewBackoff creates a new Backoff with default settings
func NewBackoff() *Backoff {
	return &Backoff{
		initialDelay: 1 * time.Second,
		maxDelay:     30 * time.Second,
		multiplier:   2.0,
		jitter:       true,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// WithInitialDelay sets the initial delay
func (b *Backoff) WithInitialDelay(d time.Duration) *Backoff {
	b.initialDelay = d
	return b
}

// WithMaxDelay sets the maximum delay
func (b *Backoff) WithMaxDelay(d time.Duration) *Backoff {
	b.maxDelay = d
	return b
}

// WithMultiplier sets the multiplier
func (b *Backoff) WithMultiplier(m float64) *Backoff {
	b.multiplier = m
	return b
}

// WithJitter enables or disables jitter
func (b *Backoff) WithJitter(j bool) *Backoff {
	b.jitter = j
	return b
}

// Next returns the next backoff duration and increments the attempt counter
func (b *Backoff) Next() time.Duration {
	delay := float64(b.initialDelay) * math.Pow(b.multiplier, float64(b.attempt))

	if delay > float64(b.maxDelay) {
		delay = float64(b.maxDelay)
	}

	if b.jitter {
		// Add +/- 20% jitter
		jitter := delay * 0.2 * (b.rng.Float64()*2 - 1)
		delay += jitter
	}

	b.attempt++

	return time.Duration(delay)
}

// Reset resets the attempt counter
func (b *Backoff) Reset() {
	b.attempt = 0
}

// Attempt returns the current attempt number
func (b *Backoff) Attempt() int {
	return b.attempt
}

// Sleep sleeps for the next backoff duration
func (b *Backoff) Sleep() {
	time.Sleep(b.Next())
}

// SleepContext sleeps for the next backoff duration or until context is cancelled
func (b *Backoff) SleepContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(b.Next()):
		return nil
	}
}

// Common retry conditions

// IsTemporaryError checks if error is temporary (implements Temporary() bool)
func IsTemporaryError(err error) bool {
	type temporary interface {
		Temporary() bool
	}
	if te, ok := err.(temporary); ok {
		return te.Temporary()
	}
	return false
}

// IsTimeoutError checks if error is a timeout
func IsTimeoutError(err error) bool {
	type timeout interface {
		Timeout() bool
	}
	if te, ok := err.(timeout); ok {
		return te.Timeout()
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// IsTransientError checks for common transient errors
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	return IsTemporaryError(err) || IsTimeoutError(err) || IsRetryable(err)
}

// RetryTransient only retries transient errors
func RetryTransient(fn func() error) error {
	config := DefaultConfig()
	config.RetryIf = IsTransientError
	return New(config).Do(fn)
}
