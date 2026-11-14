package runtime

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

// TestWorkerGroupStopAll verifies sibling cancellation starts before failing
// reader cleanup and that every loop ends before its own reader closes.
func TestWorkerGroupStopAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	started := make(chan struct{})
	siblingExited := make(chan struct{})
	failure := errors.New("processing failed")
	cleanupFailure := errors.New("flush failed")
	var closed atomic.Int32
	group := WorkerGroup{Logger: logger.NoopLogger{},
		StopOnError: true, ShutdownTimeout: time.Second,
		Workers: []Worker{
			{Name: "activity", Run: func(ctx context.Context) error {
				select {
				case <-started:
					return failure
				case <-ctx.Done():
					return nil
				}
			}, Shutdown: func(ctx context.Context) error {
				select {
				case <-siblingExited:
				case <-ctx.Done():
					return ctx.Err()
				}
				if ctx.Err() != nil {
					return errors.New("cleanup inherited cancellation")
				}
				closed.Add(1)
				return cleanupFailure
			}},
			{Name: "notification", Run: func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				close(siblingExited)
				return nil
			}, Shutdown: func(ctx context.Context) error {
				select {
				case <-siblingExited:
				default:
					return errors.New("closed while running")
				}
				closed.Add(1)
				return nil
			}},
		},
	}
	err := group.Run(ctx)
	if !errors.Is(err, failure) || !errors.Is(err, cleanupFailure) {
		t.Fatalf("errors lost: %v", err)
	}
	if closed.Load() != 2 {
		t.Fatalf("closed = %d", closed.Load())
	}
	if ctx.Err() != nil {
		t.Fatal("group cancellation affected caller or timed out")
	}
}

// TestWorkerGroupIndependent verifies a failed reader is closed promptly while
// its sibling remains active until application cancellation.
func TestWorkerGroupIndependent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	siblingCtx := make(chan context.Context, 1)
	failedClosed := make(chan struct{})
	failure := errors.New("activity failed")
	var closed atomic.Int32
	group := WorkerGroup{Logger: logger.NoopLogger{}, ShutdownTimeout: time.Second, Workers: []Worker{
		{Name: "activity", Run: func(ctx context.Context) error { return failure },
			Shutdown: func(ctx context.Context) error { close(failedClosed); closed.Add(1); return nil }},
		{Name: "notification", Run: func(ctx context.Context) error { siblingCtx <- ctx; <-ctx.Done(); return nil },
			Shutdown: func(ctx context.Context) error { closed.Add(1); return nil }},
	}}
	done := make(chan error, 1)
	go func() { done <- group.Run(ctx) }()
	var running context.Context
	select {
	case running = <-siblingCtx:
	case <-ctx.Done():
		t.Fatal("sibling did not start")
	}
	select {
	case <-failedClosed:
	case <-ctx.Done():
		t.Fatal("failed reader did not close")
	}
	if running.Err() != nil {
		t.Fatal("failed worker canceled its sibling")
	}
	select {
	case <-done:
		t.Fatal("group exited while a sibling was running")
	default:
	}
	cancel()
	if err := <-done; !errors.Is(err, failure) {
		t.Fatalf("failure lost: %v", err)
	}
	if closed.Load() != 2 {
		t.Fatal("reader cleanup did not finish")
	}
}

// TestWorkerGroupAllIndependentFailures ensures an empty service exits with
// every cause preserved, while the outer lifecycle receives only a summary.
func TestWorkerGroupAllIndependentFailures(t *testing.T) {
	first, second := errors.New("first failure"), errors.New("second failure")
	log := &loggertest.Logger{ErrorFunc: func(context.Context, string, error, ...logger.Fields) {}}
	group := WorkerGroup{ShutdownTimeout: time.Second, Logger: log, Workers: []Worker{
		{Name: "activity", Run: func(context.Context) error { return first }, Shutdown: func(context.Context) error { return nil }},
		{Name: "notification", Run: func(context.Context) error { return second }, Shutdown: func(context.Context) error { return nil }},
	}}
	err := group.Run(t.Context())
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("failures lost: %v", err)
	}
	if len(log.ErrorCalls) != 2 {
		t.Fatal("expected one detailed log per worker")
	}
	if strings.Contains(err.Error(), "first failure") || strings.Contains(err.Error(), "second failure") {
		t.Fatal("outer lifecycle would repeat failure details")
	}
}

// TestWorkerGroupUnexpectedExit ensures an early clean return still enters
// the failure policy and releases the reader.
func TestWorkerGroupUnexpectedExit(t *testing.T) {
	closed := false
	log := &loggertest.Logger{ErrorFunc: func(context.Context, string, error, ...logger.Fields) {}}
	group := WorkerGroup{StopOnError: true, ShutdownTimeout: time.Second, Logger: log, Workers: []Worker{
		{Name: "worker", Run: func(context.Context) error { return nil },
			Shutdown: func(context.Context) error { closed = true; return nil }},
	}}
	if err := group.Run(t.Context()); err == nil {
		t.Fatal("unexpected exit succeeded")
	}
	if !closed || len(log.ErrorCalls) != 1 {
		t.Fatal("failure did not release and report reader")
	}
}

// TestWorkerGroupShutdownDeadline ensures cleanup has a fresh finite budget,
// and its failure is preserved when application shutdown interrupted processing.
func TestWorkerGroupShutdownDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	group := WorkerGroup{Logger: logger.NoopLogger{}, ShutdownTimeout: 10 * time.Millisecond, Workers: []Worker{
		{Name: "worker", Run: func(ctx context.Context) error { cancel(); <-ctx.Done(); return nil },
			Shutdown: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
	}}
	if err := group.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cleanup error = %v", err)
	}
}

// TestWorkerGroupPreservesJoinedFailure protects a worker's error result even
// when it includes cancellation alongside an actual processing failure.
func TestWorkerGroupPreservesJoinedFailure(t *testing.T) {
	failure := errors.New("processing failed")
	ctx, cancel := context.WithCancel(t.Context())
	group := WorkerGroup{Logger: logger.NoopLogger{}, ShutdownTimeout: time.Second, Workers: []Worker{
		{Name: "worker", Run: func(context.Context) error {
			cancel()
			return errors.Join(context.Canceled, failure)
		}, Shutdown: func(context.Context) error { return nil }},
	}}
	err := group.Run(ctx)
	if !errors.Is(err, failure) || !errors.Is(err, context.Canceled) {
		t.Fatal("worker failure was lost")
	}
}
