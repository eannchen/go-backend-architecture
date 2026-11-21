package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/worker/workertest"
)

// TestShareWorkerOutcomes protects record decisions and processing failures;
// acknowledgments are reported independently instead of being awaited per batch.
func TestShareWorkerOutcomes(t *testing.T) {
	failure := errors.New("processing failed")
	markFailure := errors.New("mark failed")
	stopped := errors.New("end of input")
	for _, tc := range []struct {
		name         string
		handle, mark error
		outcome      string
		want         error
	}{
		{"success", nil, nil, "accept", stopped},
		{"invalid", middleware.ErrInvalidMessage, nil, "reject", stopped},
		{"processing", failure, nil, "release", failure},
		{"mark", nil, markFailure, "accept", markFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := &kgo.Record{Topic: "jobs", Offset: 1}
			var outcome string
			reader := &workertest.ShareMessageReader{FatalErrorsFunc: func() <-chan error { return nil }}
			reader.FetchRecordsFunc = func(context.Context) ([]*kgo.Record, error) {
				if len(reader.FetchRecordsCalls) > 1 {
					return nil, stopped
				}
				return []*kgo.Record{record}, nil
			}
			mark := func(name string) func(*kgo.Record) error {
				return func(got *kgo.Record) error {
					if got != record {
						t.Fatal("marked a different record")
					}
					outcome = name
					return tc.mark
				}
			}
			reader.AcceptFunc = mark("accept")
			reader.ReleaseFunc = mark("release")
			reader.RejectFunc = mark("reject")
			worker, err := NewShareWorker(reader, func(context.Context, *kgo.Record) error { return tc.handle }, ShareWorkerOptions{Concurrency: 1})
			if err != nil {
				t.Fatal(err)
			}
			if err := worker.Run(t.Context()); !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want=%v", err, tc.want)
			}
			if outcome != tc.outcome {
				t.Fatalf("outcome=%s want=%s", outcome, tc.outcome)
			}
		})
	}
}

// TestShareWorkerBoundedHandlers proves records are dispatched once through a
// bounded pool and every handler is joined before Run returns.
func TestShareWorkerBoundedHandlers(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	records := make([]*kgo.Record, 6)
	for i := range records {
		records[i] = &kgo.Record{Topic: "jobs", Offset: int64(i)}
	}
	started := make(chan struct{}, len(records))
	release := make(chan struct{})
	var active, peak, finished atomic.Int32
	stopped := errors.New("end of input")
	reader := &workertest.ShareMessageReader{FatalErrorsFunc: func() <-chan error { return nil }}
	reader.FetchRecordsFunc = func(context.Context) ([]*kgo.Record, error) {
		if len(reader.FetchRecordsCalls) > 1 {
			return nil, stopped
		}
		return records, nil
	}
	reader.AcceptFunc = func(*kgo.Record) error { return nil }
	reader.ReleaseFunc = func(*kgo.Record) error { return nil }
	handler := func(ctx context.Context, record *kgo.Record) error {
		current := active.Add(1)
		for old := peak.Load(); current > old; old = peak.Load() {
			if peak.CompareAndSwap(old, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			active.Add(-1)
			return ctx.Err()
		}
		active.Add(-1)
		finished.Add(1)
		return nil
	}
	worker, err := NewShareWorker(reader, handler, ShareWorkerOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	joined := false
	t.Cleanup(func() {
		cancel()
		if joined {
			return
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker did not stop after test cancellation")
		}
	})
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("handlers did not start")
		}
	}
	close(release)
	select {
	case err := <-done:
		joined = true
		if !errors.Is(err, stopped) {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("handlers were not joined")
	}
	if peak.Load() != 2 || active.Load() != 0 || finished.Load() != int32(len(records)) {
		t.Fatal("pool bounds or join violated")
	}
	seen := make(map[*kgo.Record]bool)
	for _, call := range reader.AcceptCalls {
		record := call.Record

		if seen[record] {
			t.Fatal("record dispatched twice")
		}
		seen[record] = true
	}
	if len(seen) != len(records) {
		t.Fatal("records missing")
	}
}

// TestShareWorkerShutdown releases unfinished records
// and waits for the cooperative handler to return before leaving Run.
func TestShareWorkerShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	finishing := make(chan struct{})
	finish := sync.OnceFunc(func() { close(finishing) })
	defer finish()
	record := &kgo.Record{Topic: "jobs"}
	reader := &workertest.ShareMessageReader{FatalErrorsFunc: func() <-chan error { return nil },
		FetchRecordsFunc: func(context.Context) ([]*kgo.Record, error) { return []*kgo.Record{record}, nil },
		ReleaseFunc:      func(*kgo.Record) error { return nil },
	}
	worker, err := NewShareWorker(reader, func(ctx context.Context, _ *kgo.Record) error {
		close(started)
		<-ctx.Done()
		<-finishing
		return ctx.Err()
	}, ShareWorkerOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	joined := false
	t.Cleanup(func() {
		cancel()
		if joined {
			return
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker did not stop after test cancellation")
		}
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	select {
	case <-done:
		joined = true
		t.Fatal("Run returned with a handler running")
	default:
	}
	finish()
	select {
	case err := <-done:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}
	if len(reader.ReleaseCalls) != 1 {
		t.Fatal("shutdown outcome incorrect")
	}
}

// TestShareWorkerFatalAcknowledgmentCancelsPoll verifies a stop request interrupts
// an idle fetch and returns its own cause instead of a batch-processing error.
func TestShareWorkerFatalAcknowledgmentCancelsPoll(t *testing.T) {
	failures := make(chan error, 1)
	fetching := make(chan struct{})
	reader := &workertest.ShareMessageReader{
		FatalErrorsFunc:  func() <-chan error { return failures },
		FetchRecordsFunc: func(ctx context.Context) ([]*kgo.Record, error) { close(fetching); <-ctx.Done(); return nil, ctx.Err() },
	}
	worker, err := NewShareWorker(reader, func(context.Context, *kgo.Record) error { return nil }, ShareWorkerOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, result := startShareWorker(t, worker)
	select {
	case <-fetching:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	failure := errors.New("acknowledgment rejected")
	failures <- failure
	select {
	case err := <-result:
		if !errors.Is(err, failure) || strings.Contains(err.Error(), "batch") {
			t.Fatalf("wrong acknowledgment failure: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("acknowledgment failure did not stop polling")
	}
}

// TestShareWorkerFatalAcknowledgmentJoinsHandlers verifies the callback's stop
// request cancels active handlers and Run waits for them to release their records.
func TestShareWorkerFatalAcknowledgmentJoinsHandlers(t *testing.T) {
	failures := make(chan error, 1)
	started := make(chan struct{}, 2)
	canceled := make(chan struct{}, 2)
	finish := make(chan struct{})
	release := sync.OnceFunc(func() { close(finish) })
	defer release()
	reader := &workertest.ShareMessageReader{
		FatalErrorsFunc:  func() <-chan error { return failures },
		FetchRecordsFunc: func(context.Context) ([]*kgo.Record, error) { return []*kgo.Record{{Offset: 0}, {Offset: 1}}, nil },
		ReleaseFunc:      func(*kgo.Record) error { return nil },
	}
	worker, err := NewShareWorker(reader, func(ctx context.Context, _ *kgo.Record) error {
		started <- struct{}{}
		<-ctx.Done()
		canceled <- struct{}{}
		<-finish
		return ctx.Err()
	}, ShareWorkerOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	ctx, result := startShareWorker(t, worker)
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	failure := errors.New("acknowledgment rejected")
	failures <- failure
	for range 2 {
		select {
		case <-canceled:
		case <-ctx.Done():
			t.Fatal("handlers were not canceled")
		}
	}
	select {
	case <-result:
		t.Fatal("Run returned while handlers were active")
	default:
	}
	release()
	select {
	case err := <-result:
		if !errors.Is(err, failure) {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("handlers did not finish")
	}
	if len(reader.ReleaseCalls) != 2 {
		t.Fatalf("released=%d want=2", len(reader.ReleaseCalls))
	}
}

func startShareWorker(t *testing.T, worker *ShareWorker) (context.Context, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	done := make(chan struct{})
	result := make(chan error, 1)
	go func() { defer close(done); result <- worker.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker did not stop")
		}
	})
	return ctx, result
}
