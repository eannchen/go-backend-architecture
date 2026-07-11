package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_SucceedsAfterRetries(t *testing.T) {
	attempts := 0

	result, err := Do(context.Background(), Options{MaxAttempts: 3}, func(_ context.Context, _ int) (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("not yet")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if result != 42 {
		t.Fatalf("result = %d, want 42", result)
	}
}

func TestDo_ReturnsLastError(t *testing.T) {
	wantErr := errors.New("boom")

	_, err := Do(context.Background(), Options{MaxAttempts: 2}, func(_ context.Context, _ int) (int, error) {
		return 0, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Do() error = %v, want %v", err, wantErr)
	}
}

func TestDo_StopsWhenShouldRetryReturnsFalse(t *testing.T) {
	wantErr := errors.New("do not retry")
	attempts := 0

	_, err := Do(context.Background(), Options{
		MaxAttempts: 5,
		ShouldRetry: func(err error) bool {
			return !errors.Is(err, wantErr)
		},
	}, func(_ context.Context, _ int) (int, error) {
		attempts++
		return 0, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Do() error = %v, want %v", err, wantErr)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestDo_CallsOnRetry(t *testing.T) {
	var calls int
	var gotDelay time.Duration

	_, err := Do(context.Background(), Options{
		MaxAttempts: 2,
		Delay:       10 * time.Millisecond,
		OnRetry: func(_ int, _ error, delay time.Duration) {
			calls++
			gotDelay = delay
		},
	}, func(_ context.Context, _ int) (int, error) {
		return 0, errors.New("retry")
	})
	if err == nil {
		t.Fatal("Do() error = nil, want retry error")
	}
	if calls != 1 {
		t.Fatalf("retry callbacks = %d, want 1", calls)
	}
	if gotDelay != 10*time.Millisecond {
		t.Fatalf("retry delay = %v, want 10ms", gotDelay)
	}
}

func TestDo_StopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	attempts := 0
	_, err := Do(ctx, Options{MaxAttempts: 3}, func(_ context.Context, _ int) (int, error) {
		attempts++
		cancel()
		return 0, errors.New("retry")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context cancellation", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestSleep_ReturnsFalseWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if Sleep(ctx, time.Second) {
		t.Fatal("Sleep() = true, want false for canceled context")
	}
}
