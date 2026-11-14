package contextmw

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestContextDeadline protects caller cancellation, the earlier parent deadline,
// late nil results, and release of the child context after handling.
func TestContextDeadline(t *testing.T) {
	for _, mode := range []string{"success", "parent deadline", "canceled", "late success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			parentDeadline := time.Now().Add(10 * time.Second)
			if mode == "parent deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(ctx, parentDeadline)
				defer stop()
			}
			if mode == "canceled" {
				cancel()
			}
			timeout := time.Minute
			if mode == "late success" {
				timeout = 10 * time.Millisecond
			}
			failure := errors.New("handler failed")
			var handling context.Context
			handler := New(nil, Config{HandlingTimeout: timeout, AcknowledgmentMargin: time.Hour}).Handler()(func(ctx context.Context, _ *kgo.Record) error {
				handling = ctx
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Error("missing deadline")
				}
				if mode == "parent deadline" && !deadline.Equal(parentDeadline) {
					t.Error("parent deadline extended")
				}
				if mode == "late success" {
					<-ctx.Done()
				}
				if mode == "failure" {
					return failure
				}
				return nil
			})
			err := handler(ctx, &kgo.Record{})
			switch mode {
			case "canceled":
				if !errors.Is(err, context.Canceled) || handling != nil {
					t.Fatal("canceled message reached handler")
				}
			case "late success":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("late success accepted")
				}
			case "failure":
				if !errors.Is(err, failure) {
					t.Fatal("handler failure lost")
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
			if handling != nil && handling.Err() == nil {
				t.Fatal("child context not released")
			}
		})
	}
}

// TestRecordTraceContext protects kotel-context transfer and header fallback
// while preserving the worker's cancellation and local context values.
func TestRecordTraceContext(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "headers"
		if native {
			name = "record context"
		}
		t.Run(name, func(t *testing.T) {
			type key struct{}
			worker, cancel := context.WithCancel(context.WithValue(t.Context(), key{}, "worker"))
			defer cancel()
			recordCtx := context.WithValue(t.Context(), key{}, "record")
			tracer := &observabilitytest.Tracer{
				TraceContextFunc: func(ctx context.Context) (observability.TraceContext, bool) {
					return observability.TraceContext{}, native
				},
				InjectFunc: func(ctx context.Context, carrier observability.TextMapCarrier) {
					if ctx != recordCtx {
						t.Error("wrong native context")
					}
					carrier.Set("traceparent", "native")
				},
				ExtractFunc: func(ctx context.Context, carrier observability.TextMapCarrier) context.Context {
					want := "header"
					if native {
						want = "native"
					}
					if carrier.Get("traceparent") != want || ctx.Value(key{}) != "worker" {
						t.Error("wrong propagation source")
					}
					return ctx
				},
			}
			record := &kgo.Record{Headers: []kgo.RecordHeader{{Key: "traceparent", Value: []byte("header")}}}
			if native {
				record.Context = recordCtx
			}
			ctx := recordContext(worker, tracer, record)
			cancel()
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatal("worker cancellation lost")
			}
			if tracer.ExtractCalls != 1 {
				t.Fatal("trace not extracted")
			}
		})
	}
}
