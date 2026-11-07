package kafkaprocessing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestWrapperSharesPolicyAcrossHandlers protects common recovery/deadline policy
// while keeping each handler's operation identity separate in processing metrics.
func TestWrapperSharesPolicyAcrossHandlers(t *testing.T) {
	meter := observabilitytest.NewRecordingMeter()
	wrapper := New(nil, nil, meter, Config{HandlingTimeout: 20 * time.Millisecond})
	panicHandler := wrapper.Wrap("notifications.consume", func(context.Context, *kgo.Record) error { panic("private payload") })
	timeoutHandler := wrapper.Wrap("reports.consume", func(ctx context.Context, _ *kgo.Record) error { <-ctx.Done(); return nil })
	if err := panicHandler(t.Context(), &kgo.Record{}); !errors.Is(err, middleware.ErrHandlerPanic) {
		t.Fatalf("panic outcome=%v", err)
	}
	if err := timeoutHandler(t.Context(), &kgo.Record{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout outcome=%v", err)
	}
	samples := meter.CounterSamples("app.kafka.processing.attempts")
	if len(samples) != 2 {
		t.Fatalf("samples=%v", samples)
	}
	for i, want := range []struct{ operation, outcome string }{{"notifications.consume", "panic"}, {"reports.consume", "timeout"}} {
		if samples[i].Fields["app.kafka.operation"] != want.operation || samples[i].Fields["app.kafka.outcome"] != want.outcome {
			t.Fatalf("sample=%v want=%v", samples[i], want)
		}
	}
}
