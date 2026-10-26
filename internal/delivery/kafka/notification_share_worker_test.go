package kafka_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/kafkatest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification/notificationtest"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestShareWorkerBoundedOutOfOrderHandling protects concurrency within a single
// partition and ensures the next poll waits for every outcome and its flush.
func TestShareWorkerBoundedOutOfOrderHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	records := []*kgo.Record{shareRecord(0), shareRecord(1), shareRecord(2)}
	batch := successfulBatch(records)
	gates := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	started := make(chan int, 3)
	accepted := make(chan int64, 3)
	var active, peak atomic.Int32
	usecase := &notificationtest.Usecase{HandleFunc: func(ctx context.Context, r notification.Request) error {
		var n int
		_, err := fmt.Sscanf(r.RequestID, "req-%d", &n)
		if err != nil {
			return err
		}
		count := active.Add(1)
		for old := peak.Load(); count > old; old = peak.Load() {
			if peak.CompareAndSwap(old, count) {
				break
			}
		}
		defer active.Add(-1)
		started <- n
		select {
		case <-gates[n]:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	batch.AcceptFunc = func(r *kgo.Record) error { accepted <- r.Offset; return nil }
	reader := &kafkatest.ShareReader{}
	reader.FetchBatchFunc = func(context.Context) (kafkadelivery.ShareWorkBatch, error) {
		if reader.FetchBatchCalls == 1 {
			return batch, nil
		}
		if len(batch.AcceptCalls) != 3 || reader.FlushAcknowledgmentsCalls != 1 {
			return nil, errors.New("polled before completing and flushing batch")
		}
		stopWorker()
		return nil, workerCtx.Err()
	}
	reader.FlushAcknowledgmentsFunc = func(context.Context) error { return nil }
	worker := newShareWorker(t, reader, usecase, 2)
	done := make(chan error, 1)
	joined := make(chan struct{})
	go func() { defer close(joined); done <- worker.Run(workerCtx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Error("worker did not stop")
		}
	})
	first, second := receive(t, ctx, started), receive(t, ctx, started)
	if first == second {
		t.Fatal("same record started twice")
	}
	// Keep one handler blocked while the other completes and takes queued work.
	close(gates[second])
	if got := receive(t, ctx, accepted); got != int64(second) {
		t.Fatalf("accepted=%d want=%d", got, second)
	}
	third := receive(t, ctx, started)
	if third == first || third == second {
		t.Fatal("duplicate dispatch")
	}
	close(gates[third])
	receive(t, ctx, accepted)
	close(gates[first])
	receive(t, ctx, accepted)
	err := receive(t, ctx, done)
	if err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 2 {
		t.Fatalf("peak handlers=%d", peak.Load())
	}
}

// TestShareWorkerOutcomes protects accept/reject/release policy and preserves
// processing and acknowledgment errors for the lifecycle owner.
func TestShareWorkerOutcomes(t *testing.T) {
	failure := errors.New("dependency failure")
	for _, tc := range []struct {
		name                       string
		payload                    []byte
		handle                     func(context.Context, notification.Request) error
		markErr, flushErr, wantErr error
		want                       string
	}{
		{name: "success", want: "accept"},
		{name: "malformed", payload: []byte("private payload"), want: "reject"},
		{name: "missing field", payload: []byte(`{"request_id":"r"}`), want: "reject"},
		{name: "usecase failure", handle: func(context.Context, notification.Request) error { return failure }, want: "release", wantErr: failure},
		{name: "late nil return", handle: func(ctx context.Context, _ notification.Request) error { <-ctx.Done(); return nil }, want: "release", wantErr: context.DeadlineExceeded},
		{name: "timeout", handle: func(ctx context.Context, _ notification.Request) error { <-ctx.Done(); return ctx.Err() }, want: "release", wantErr: context.DeadlineExceeded},
		{name: "panic", handle: func(context.Context, notification.Request) error { panic("secret panic") }, want: "release"},
		{name: "mark failure", want: "accept", markErr: failure, wantErr: failure},
		{name: "flush failure", want: "accept", flushErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			record := shareRecord(0)
			if tc.payload != nil {
				record.Value = tc.payload
			}
			batch := successfulBatch([]*kgo.Record{record})
			batch.AcceptFunc = func(*kgo.Record) error { return tc.markErr }
			reader := &kafkatest.ShareReader{}
			reader.FetchBatchFunc = func(context.Context) (kafkadelivery.ShareWorkBatch, error) {
				if reader.FetchBatchCalls == 1 {
					return batch, nil
				}
				cancel()
				return nil, ctx.Err()
			}
			reader.FlushAcknowledgmentsFunc = func(context.Context) error { return tc.flushErr }
			handle := tc.handle
			if handle == nil {
				handle = func(context.Context, notification.Request) error { return nil }
			}
			worker, err := kafkadelivery.NewNotificationShareWorker(reader, &notificationtest.Usecase{HandleFunc: handle}, nil, kafkadelivery.ShareWorkerOptions{Concurrency: 1, HandlingTimeout: 20 * time.Millisecond, AcknowledgmentMargin: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			err = worker.Run(ctx)
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error=%v want=%v", err, tc.wantErr)
			}
			if tc.name == "panic" && (err == nil || strings.Contains(err.Error(), "secret")) {
				t.Fatalf("panic error=%v", err)
			}
			if tc.wantErr == nil && tc.name != "panic" && err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{"accept": len(batch.AcceptCalls), "release": len(batch.ReleaseCalls), "reject": len(batch.RejectCalls)}
			for name, count := range counts {
				want := 0
				if name == tc.want {
					want = 1
				}
				if count != want {
					t.Fatalf("outcomes=%v want=%s", counts, tc.want)
				}
			}
			if tc.want == "reject" && reader.FetchBatchCalls != 2 {
				t.Fatal("invalid message stopped the worker")
			}
			if reader.FlushAcknowledgmentsCalls != 1 {
				t.Fatal("decisions were not flushed")
			}
		})
	}
}

// TestShareWorkerShutdownJoinsHandlers ensures shutdown cannot close the reader
// while a handler is still running, and queued work is released without invoking it.
func TestShareWorkerShutdownJoinsHandlers(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, canceled, finish := make(chan struct{}), make(chan struct{}), make(chan struct{})
	batch := successfulBatch([]*kgo.Record{shareRecord(0), shareRecord(1), shareRecord(2)})
	reader := &kafkatest.ShareReader{FetchBatchFunc: func(context.Context) (kafkadelivery.ShareWorkBatch, error) { return batch, nil }}
	usecase := &notificationtest.Usecase{HandleFunc: func(ctx context.Context, _ notification.Request) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-finish
		return ctx.Err()
	}}
	worker := newShareWorker(t, reader, usecase, 1)
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	waitCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	receive(t, waitCtx, started)
	cancel()
	receive(t, waitCtx, canceled)
	select {
	case err := <-done:
		t.Fatalf("returned while handler still runs: %v", err)
	default:
	}
	close(finish)
	if err := receive(t, waitCtx, done); err != nil {
		t.Fatal(err)
	}
	if len(usecase.HandleCalls) != 1 || len(batch.ReleaseCalls) != 3 || reader.FlushAcknowledgmentsCalls != 0 {
		t.Fatalf("calls=%d releases=%d flushes=%d", len(usecase.HandleCalls), len(batch.ReleaseCalls), reader.FlushAcknowledgmentsCalls)
	}
}

// TestShareWorkerFetchAndOptions rejects invalid pool settings and preserves
// fetch failures, while cancellation of an idle fetch is normal shutdown.
func TestShareWorkerFetchAndOptions(t *testing.T) {
	valid := kafkadelivery.ShareWorkerOptions{Concurrency: 2, HandlingTimeout: time.Second, AcknowledgmentMargin: time.Second}
	for _, option := range []kafkadelivery.ShareWorkerOptions{{}, {Concurrency: 1001, HandlingTimeout: time.Second, AcknowledgmentMargin: time.Second}, {Concurrency: 1, HandlingTimeout: time.Second}} {
		if _, err := kafkadelivery.NewNotificationShareWorker(nil, nil, nil, option); err == nil {
			t.Fatal("accepted invalid options")
		}
	}
	for _, stopping := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		failure := errors.New("broker unavailable")
		reader := &kafkatest.ShareReader{FetchBatchFunc: func(context.Context) (kafkadelivery.ShareWorkBatch, error) {
			if stopping {
				cancel()
				return nil, ctx.Err()
			}
			return nil, failure
		}}
		worker, err := kafkadelivery.NewNotificationShareWorker(reader, nil, nil, valid)
		if err != nil {
			t.Fatal(err)
		}
		err = worker.Run(ctx)
		cancel()
		if stopping && err != nil {
			t.Fatal(err)
		}
		if !stopping && !errors.Is(err, failure) {
			t.Fatalf("lost fetch error: %v", err)
		}
	}
}

func shareRecord(n int) *kgo.Record {
	return &kgo.Record{Topic: "notifications", Partition: 0, Offset: int64(n), Value: []byte(fmt.Sprintf(`{"request_id":"req-%d","recipient":"user","content":"ready"}`, n))}
}
func successfulBatch(records []*kgo.Record) *kafkatest.ShareBatch {
	return &kafkatest.ShareBatch{RecordsFunc: func() []*kgo.Record { return records }, AcceptFunc: func(*kgo.Record) error { return nil }, ReleaseFunc: func(*kgo.Record) error { return nil }, RejectFunc: func(*kgo.Record) error { return nil }}
}
func newShareWorker(t *testing.T, reader kafkadelivery.ShareMessageReader, usecase notification.Usecase, concurrency int) *kafkadelivery.NotificationShareWorker {
	t.Helper()
	worker, err := kafkadelivery.NewNotificationShareWorker(reader, usecase, nil, kafkadelivery.ShareWorkerOptions{Concurrency: concurrency, HandlingTimeout: 10 * time.Second, AcknowledgmentMargin: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return worker
}
func receive[T any](t *testing.T, ctx context.Context, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-ctx.Done():
		t.Fatal(ctx.Err())
		var zero T
		return zero
	}
}
