package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestCleanupAfterInitFailureFreshContext protects cleanup from failed startup
// contexts while retaining correlation values and releasing the cleanup timer.
func TestCleanupAfterInitFailureFreshContext(t *testing.T) {
	type contextKey struct{}
	for _, state := range []string{"active", "canceled", "expired", "future deadline"} {
		t.Run(state, func(t *testing.T) {
			parent := context.WithValue(context.Background(), contextKey{}, "startup-trace")
			var cancel context.CancelFunc
			switch state {
			case "expired":
				parent, cancel = context.WithDeadline(parent, time.Now().Add(-time.Second))
			case "future deadline":
				parent, cancel = context.WithTimeout(parent, time.Hour)
			default:
				parent, cancel = context.WithCancel(parent)
			}
			defer cancel()
			if state == "canceled" {
				cancel()
			}

			cleanupFailure := errors.New("flush failed")
			const budget = time.Minute
			before := time.Now()
			var cleanupCtx context.Context
			err := CleanupAfterInitFailure(parent, budget, func(ctx context.Context) error {
				cleanupCtx = ctx
				if ctx.Err() != nil {
					t.Fatalf("cleanup inherited startup failure: %v", ctx.Err())
				}
				if got := ctx.Value(contextKey{}); got != "startup-trace" {
					t.Fatalf("context value = %v, want startup-trace", got)
				}
				deadline, ok := ctx.Deadline()
				if !ok || deadline.Before(before.Add(budget)) || deadline.After(time.Now().Add(budget)) {
					t.Fatalf("cleanup deadline = %v (present=%v), want fresh %v budget", deadline, ok, budget)
				}
				// Cancel during cleanup too: it must have an independent lifetime.
				cancel()
				if ctx.Err() != nil {
					t.Fatalf("startup cancellation interrupted cleanup: %v", ctx.Err())
				}
				return cleanupFailure
			})
			if !errors.Is(err, cleanupFailure) {
				t.Fatalf("cleanup error = %v, want original failure", err)
			}
			if cleanupCtx == nil || !errors.Is(cleanupCtx.Err(), context.Canceled) {
				t.Fatal("cleanup context was not canceled after callback returned")
			}
		})
	}
}

// TestCleanupAfterInitFailureDeadline checks cooperative cleanup is interrupted
// by its own budget even when the startup context has no deadline.
func TestCleanupAfterInitFailureDeadline(t *testing.T) {
	err := CleanupAfterInitFailure(context.Background(), 10*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			t.Fatal("cleanup deadline did not cancel the callback")
			return nil
		}
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cleanup error = %v, want deadline exceeded", err)
	}
}
