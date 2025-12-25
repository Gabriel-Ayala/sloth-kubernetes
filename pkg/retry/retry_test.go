package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, want 1s", config.InitialDelay)
	}
	if config.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want 30s", config.MaxDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", config.Multiplier)
	}
	if !config.Jitter {
		t.Error("Jitter should be true")
	}
}

func TestAggressiveConfig(t *testing.T) {
	config := AggressiveConfig()

	if config.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", config.MaxRetries)
	}
	if config.InitialDelay != 500*time.Millisecond {
		t.Errorf("InitialDelay = %v, want 500ms", config.InitialDelay)
	}
}

func TestQuickConfig(t *testing.T) {
	config := QuickConfig()

	if config.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", config.MaxRetries)
	}
	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("InitialDelay = %v, want 100ms", config.InitialDelay)
	}
}

func TestRetryableError(t *testing.T) {
	originalErr := errors.New("original error")
	retryableErr := NewRetryableError(originalErr)

	if retryableErr == nil {
		t.Fatal("NewRetryableError returned nil")
	}

	if !IsRetryable(retryableErr) {
		t.Error("IsRetryable should return true for RetryableError")
	}

	if IsRetryable(originalErr) {
		t.Error("IsRetryable should return false for regular error")
	}

	if retryableErr.Error() != "original error" {
		t.Errorf("Error() = %q, want %q", retryableErr.Error(), "original error")
	}

	// Test unwrap
	if !errors.Is(retryableErr, originalErr) {
		t.Error("errors.Is should match original error")
	}
}

func TestNewRetryableErrorNil(t *testing.T) {
	if NewRetryableError(nil) != nil {
		t.Error("NewRetryableError(nil) should return nil")
	}
}

func TestDoSuccess(t *testing.T) {
	attempts := 0
	err := Do(func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("Do returned error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	config := QuickConfig()
	config.MaxRetries = 3

	attempts := 0
	err := New(config).Do(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Do returned error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestDoMaxRetriesExceeded(t *testing.T) {
	config := QuickConfig()
	config.MaxRetries = 2

	attempts := 0
	err := New(config).Do(func() error {
		attempts++
		return errors.New("persistent error")
	})

	if err == nil {
		t.Error("Do should return error when max retries exceeded")
	}
	// Initial attempt + 2 retries = 3 attempts
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestDoWithContext(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	err := DoWithContext(ctx, func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("DoWithContext returned error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestDoWithContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := QuickConfig()
	err := New(config).DoWithContext(ctx, func() error {
		return errors.New("should not reach here")
	})

	if err == nil {
		t.Error("DoWithContext should return error when context cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should wrap context.Canceled, got: %v", err)
	}
}

func TestDoWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	config := Config{
		MaxRetries:   10,
		InitialDelay: 100 * time.Millisecond, // Longer than context timeout
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	attempts := 0
	err := New(config).DoWithContext(ctx, func() error {
		attempts++
		return errors.New("keep retrying")
	})

	if err == nil {
		t.Error("DoWithContext should return error on timeout")
	}
}

func TestRetryIf(t *testing.T) {
	config := QuickConfig()
	config.RetryIf = func(err error) bool {
		return err.Error() == "retry me"
	}

	// Test error that should be retried
	attempts := 0
	err := New(config).Do(func() error {
		attempts++
		if attempts < 2 {
			return errors.New("retry me")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Do returned error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}

	// Test error that should not be retried
	attempts = 0
	err = New(config).Do(func() error {
		attempts++
		return errors.New("do not retry")
	})

	if err == nil {
		t.Error("Do should return error for non-retryable error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (should not retry)", attempts)
	}
}

func TestOnRetryCallback(t *testing.T) {
	config := QuickConfig()
	config.MaxRetries = 2

	var callbacks []int
	config.OnRetry = func(attempt int, err error, delay time.Duration) {
		callbacks = append(callbacks, attempt)
	}

	New(config).Do(func() error {
		return errors.New("keep failing")
	})

	if len(callbacks) != 2 {
		t.Errorf("OnRetry called %d times, want 2", len(callbacks))
	}
	if len(callbacks) >= 2 && (callbacks[0] != 1 || callbacks[1] != 2) {
		t.Errorf("OnRetry attempts = %v, want [1, 2]", callbacks)
	}
}

func TestWithRetries(t *testing.T) {
	attempts := 0
	err := WithRetries(5, func() error {
		attempts++
		if attempts < 4 {
			return errors.New("temporary")
		}
		return nil
	})

	if err != nil {
		t.Errorf("WithRetries returned error: %v", err)
	}
	if attempts != 4 {
		t.Errorf("attempts = %d, want 4", attempts)
	}
}

func TestDoWithData(t *testing.T) {
	config := QuickConfig()
	r := New(config)

	attempts := 0
	result, err := DoWithData(r, func() (string, error) {
		attempts++
		if attempts < 2 {
			return "", errors.New("temporary")
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("DoWithData returned error: %v", err)
	}
	if result != "success" {
		t.Errorf("result = %q, want %q", result, "success")
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestDoWithDataContext(t *testing.T) {
	ctx := context.Background()
	config := QuickConfig()
	r := New(config)

	result, err := DoWithDataContext(ctx, r, func() (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Errorf("DoWithDataContext returned error: %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
}

func TestBackoff(t *testing.T) {
	b := NewBackoff().
		WithInitialDelay(100 * time.Millisecond).
		WithMaxDelay(1 * time.Second).
		WithMultiplier(2.0).
		WithJitter(false)

	// First delay should be ~100ms
	d1 := b.Next()
	if d1 != 100*time.Millisecond {
		t.Errorf("first delay = %v, want 100ms", d1)
	}

	// Second delay should be ~200ms
	d2 := b.Next()
	if d2 != 200*time.Millisecond {
		t.Errorf("second delay = %v, want 200ms", d2)
	}

	// Third delay should be ~400ms
	d3 := b.Next()
	if d3 != 400*time.Millisecond {
		t.Errorf("third delay = %v, want 400ms", d3)
	}

	if b.Attempt() != 3 {
		t.Errorf("Attempt() = %d, want 3", b.Attempt())
	}
}

func TestBackoffMaxDelay(t *testing.T) {
	b := NewBackoff().
		WithInitialDelay(100 * time.Millisecond).
		WithMaxDelay(500 * time.Millisecond).
		WithMultiplier(10.0).
		WithJitter(false)

	// First: 100ms
	b.Next()
	// Second: 1000ms -> capped to 500ms
	d := b.Next()
	if d != 500*time.Millisecond {
		t.Errorf("delay = %v, want 500ms (capped)", d)
	}
}

func TestBackoffReset(t *testing.T) {
	b := NewBackoff().
		WithInitialDelay(100 * time.Millisecond).
		WithJitter(false)

	b.Next()
	b.Next()
	b.Next()

	if b.Attempt() != 3 {
		t.Errorf("Attempt() before reset = %d, want 3", b.Attempt())
	}

	b.Reset()

	if b.Attempt() != 0 {
		t.Errorf("Attempt() after reset = %d, want 0", b.Attempt())
	}

	d := b.Next()
	if d != 100*time.Millisecond {
		t.Errorf("delay after reset = %v, want 100ms", d)
	}
}

func TestBackoffWithJitter(t *testing.T) {
	b := NewBackoff().
		WithInitialDelay(1 * time.Second).
		WithJitter(true)

	// Run multiple times and check that values vary
	delays := make(map[time.Duration]bool)
	for i := 0; i < 10; i++ {
		b.Reset()
		d := b.Next()
		delays[d] = true
	}

	// With jitter, we should get different values
	if len(delays) < 2 {
		t.Error("Jitter should produce varying delays")
	}
}

func TestBackoffSleepContext(t *testing.T) {
	b := NewBackoff().WithInitialDelay(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	// Normal sleep
	start := time.Now()
	err := b.SleepContext(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("SleepContext returned error: %v", err)
	}
	if elapsed < 10*time.Millisecond {
		t.Errorf("SleepContext returned too quickly: %v", elapsed)
	}

	// Cancelled context
	cancel()
	err = b.SleepContext(ctx)
	if err == nil {
		t.Error("SleepContext should return error when context cancelled")
	}
}

func TestIsTemporaryError(t *testing.T) {
	// Regular error - not temporary
	regularErr := errors.New("regular error")
	if IsTemporaryError(regularErr) {
		t.Error("regular error should not be temporary")
	}

	// Temporary error
	tempErr := &temporaryError{temp: true}
	if !IsTemporaryError(tempErr) {
		t.Error("temporary error should be detected")
	}

	// Non-temporary error with interface
	nonTempErr := &temporaryError{temp: false}
	if IsTemporaryError(nonTempErr) {
		t.Error("non-temporary error should not be detected as temporary")
	}
}

type temporaryError struct {
	temp bool
}

func (e *temporaryError) Error() string   { return "temporary error" }
func (e *temporaryError) Temporary() bool { return e.temp }

func TestIsTimeoutError(t *testing.T) {
	// Regular error - not timeout
	regularErr := errors.New("regular error")
	if IsTimeoutError(regularErr) {
		t.Error("regular error should not be timeout")
	}

	// Timeout error
	timeoutErr := &timeoutError{timeout: true}
	if !IsTimeoutError(timeoutErr) {
		t.Error("timeout error should be detected")
	}

	// Context deadline exceeded
	if !IsTimeoutError(context.DeadlineExceeded) {
		t.Error("DeadlineExceeded should be detected as timeout")
	}
}

type timeoutError struct {
	timeout bool
}

func (e *timeoutError) Error() string { return "timeout error" }
func (e *timeoutError) Timeout() bool { return e.timeout }

func TestIsTransientError(t *testing.T) {
	if IsTransientError(nil) {
		t.Error("nil should not be transient")
	}

	if IsTransientError(errors.New("regular")) {
		t.Error("regular error should not be transient")
	}

	if !IsTransientError(&temporaryError{temp: true}) {
		t.Error("temporary error should be transient")
	}

	if !IsTransientError(&timeoutError{timeout: true}) {
		t.Error("timeout error should be transient")
	}

	if !IsTransientError(NewRetryableError(errors.New("retryable"))) {
		t.Error("retryable error should be transient")
	}
}

func TestRetryTransient(t *testing.T) {
	// Non-transient error should not retry
	attempts := 0
	err := RetryTransient(func() error {
		attempts++
		return errors.New("not transient")
	})

	if err == nil {
		t.Error("RetryTransient should return error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (should not retry)", attempts)
	}

	// Transient error should retry
	attempts = 0
	err = RetryTransient(func() error {
		attempts++
		if attempts < 2 {
			return NewRetryableError(errors.New("transient"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("RetryTransient returned error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestCalculateDelay(t *testing.T) {
	config := Config{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}
	r := New(config)

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // Capped
		{6, 30 * time.Second}, // Capped
	}

	for _, tt := range tests {
		got := r.calculateDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("calculateDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestZeroRetries(t *testing.T) {
	config := DefaultConfig()
	config.MaxRetries = 0

	attempts := 0
	err := New(config).Do(func() error {
		attempts++
		return errors.New("fail")
	})

	if err == nil {
		t.Error("should return error with 0 retries")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRetrierReuse(t *testing.T) {
	config := QuickConfig()
	r := New(config)

	// First use
	attempts1 := 0
	r.Do(func() error {
		attempts1++
		if attempts1 < 2 {
			return errors.New("fail")
		}
		return nil
	})

	// Second use (should start fresh)
	attempts2 := 0
	r.Do(func() error {
		attempts2++
		return nil
	})

	if attempts1 != 2 {
		t.Errorf("first run attempts = %d, want 2", attempts1)
	}
	if attempts2 != 1 {
		t.Errorf("second run attempts = %d, want 1", attempts2)
	}
}
