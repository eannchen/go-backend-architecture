package retry

import (
	"context"
	"time"
)

type Options struct {
	MaxAttempts int
	Delay       time.Duration
	OnRetry     func(attempt int, err error, delay time.Duration)
	// ShouldRetry is optional. When non-nil and it returns false, Do returns the error immediately.
	ShouldRetry func(err error) bool
}

// Do executes fn up to MaxAttempts times until it succeeds or the context ends.
func Do[T any](ctx context.Context, opts Options, fn func(context.Context, int) (T, error)) (T, error) {
	var zero T

	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		value, err := fn(ctx, attempt)
		if err == nil {
			return value, nil
		}
		lastErr = err

		if opts.ShouldRetry != nil && !opts.ShouldRetry(err) {
			return zero, err
		}
		if attempt == maxAttempts {
			break
		}

		if opts.OnRetry != nil {
			opts.OnRetry(attempt, err, opts.Delay)
		}
		if !Sleep(ctx, opts.Delay) {
			return zero, ctx.Err()
		}
	}

	return zero, lastErr
}

// Sleep waits for d or returns false when the context ends first.
func Sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
