package worker

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/worker/workertest"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestWorkerProcessingOrder protects manual progress: handle before commit,
// always unblock rebalances, and stop without committing failed work.
func TestWorkerProcessingOrder(t *testing.T) {
	failure := errors.New("operation failed")
	stopped := errors.New("end of input")
	for _, stage := range []string{"success", "fetch", "handle", "commit"} {
		t.Run(stage, func(t *testing.T) {
			record := &kgo.Record{Topic: "mail", Partition: 1, Offset: 8}
			var events []string
			reader := &workertest.MessageReader{}
			reader.FetchRecordFunc = func(context.Context) (*kgo.Record, error) {
				events = append(events, "fetch")
				if reader.FetchRecordCalls > 1 {
					return nil, stopped
				}
				if stage == "fetch" {
					return nil, failure
				}
				return record, nil
			}
			reader.CommitRecordsFunc = func(ctx context.Context, records ...*kgo.Record) error {
				events = append(events, "commit")
				if ctx != t.Context() || len(records) != 1 || records[0] != record {
					t.Fatal("commit changed context or record")
				}
				if stage == "commit" {
					return failure
				}
				return nil
			}
			reader.AllowRebalanceFunc = func() { events = append(events, "rebalance") }
			handler := func(ctx context.Context, got *kgo.Record) error {
				events = append(events, "handle")
				if ctx != t.Context() || got != record {
					t.Fatal("handler changed context or record")
				}
				if stage == "handle" {
					return failure
				}
				return nil
			}
			err := NewWorker(reader, handler).Run(t.Context())
			want := []string{"fetch"}
			switch stage {
			case "success":
				want = []string{"fetch", "handle", "commit", "rebalance", "fetch"}
				if !errors.Is(err, stopped) {
					t.Fatal(err)
				}
			case "handle":
				want = []string{"fetch", "handle", "rebalance"}
			case "commit":
				want = []string{"fetch", "handle", "commit", "rebalance"}
			}
			if stage != "success" && !errors.Is(err, failure) {
				t.Fatal(err)
			}
			if !slices.Equal(events, want) {
				t.Fatalf("events = %v, want %v", events, want)
			}
		})
	}
}

// TestWorkerCancellation exits normally when shutdown interrupts a blocking
// fetch, with no processing or progress change.
func TestWorkerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	reader := &workertest.MessageReader{FetchRecordFunc: func(ctx context.Context) (*kgo.Record, error) { cancel(); return nil, ctx.Err() }}
	err := NewWorker(reader, func(context.Context, *kgo.Record) error { t.Fatal("handler invoked"); return nil }).Run(ctx)
	if err != nil || reader.AllowRebalanceCalls != 0 || len(reader.CommitRecordsCalls) != 0 {
		t.Fatalf("shutdown = %v", err)
	}
}

// TestWorkerCancellationDuringProcessing protects unfinished work from being
// committed when shutdown interrupts handling, while releasing rebalance holds.
func TestWorkerCancellationDuringProcessing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader := &workertest.MessageReader{
		FetchRecordFunc:    func(context.Context) (*kgo.Record, error) { return &kgo.Record{Topic: "mail"}, nil },
		AllowRebalanceFunc: func() {},
	}
	handler := func(ctx context.Context, _ *kgo.Record) error { cancel(); return ctx.Err() }
	if err := NewWorker(reader, handler).Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(reader.CommitRecordsCalls) != 0 || reader.AllowRebalanceCalls != 1 {
		t.Fatal("unfinished work advanced progress or retained ownership")
	}
}
