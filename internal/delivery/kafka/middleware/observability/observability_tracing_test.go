package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/codes"
	observabilitymetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"
	recoverymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	"github.com/eannchen/go-backend-architecture/pkg/logger"
	contracts "github.com/eannchen/go-backend-architecture/pkg/observability"
	"github.com/eannchen/go-backend-architecture/pkg/observability/observabilitytest"
	"github.com/eannchen/go-backend-architecture/pkg/observability/otel"
)

// TestProcessingTracing protects the processing span context and failure
// completion, while excluding message keys and payloads from span attributes.
func TestProcessingTracing(t *testing.T) {
	type key struct{}
	span := &observabilitytest.Span{
		SetAttributesFunc: func(...contracts.Fields) {},
		FinishFunc:        func(error, ...string) {},
	}
	tracer := &observabilitytest.Tracer{StartFunc: func(ctx context.Context, scope, name string, _ ...contracts.Fields) (context.Context, contracts.Span) {
		if scope != "delivery.kafka" || name != "activity.consume" {
			t.Error("wrong span identity")
		}
		return context.WithValue(ctx, key{}, "processing"), span
	}}
	failure := errors.New("workflow failed")
	handler := New(tracer, logger.NoopLogger{}, contracts.NoopMeter{}).Handler("activity.consume")(func(ctx context.Context, _ *kgo.Record) error {
		if ctx.Value(key{}) != "processing" {
			t.Error("span context not passed to handler")
		}
		return failure
	})
	err := handler(t.Context(), &kgo.Record{Topic: "activities", Partition: 2, Offset: 7, Key: []byte("private-key"), Value: []byte("private-body")})
	if !errors.Is(err, failure) || len(tracer.StartCalls) != 1 || len(span.FinishCalls) != 1 || !errors.Is(span.FinishCalls[0].Err, failure) {
		t.Fatal("span lifecycle lost processing failure")
	}
	if len(span.SetAttributesCalls) != 1 || span.SetAttributesCalls[0].Fields[0]["app.kafka.outcome"] != "error" {
		t.Fatal("span outcome missing")
	}
	if strings.Contains(fmt.Sprint(tracer.StartCalls[0].Fields), "private-") {
		t.Fatal("private data included in span")
	}
}

// TestKafkaProcessingContinuesReceiveSpan verifies the real tracing adapter
// parents processing to kotel's ended receive span and keeps worker cancellation.
func TestKafkaProcessingContinuesReceiveSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	for _, withHeaders := range []bool{false, true} {
		t.Run(map[bool]string{false: "new trace", true: "propagated trace"}[withHeaders], func(t *testing.T) {
			record := &kgo.Record{Topic: "notifications"}
			parentCtx, parent := provider.Tracer("test").Start(t.Context(), "publisher")
			if withHeaders {
				carrier := propagation.MapCarrier{}
				propagation.TraceContext{}.Inject(parentCtx, carrier)
				for key, value := range carrier {
					record.Headers = append(record.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
				}
			}
			parent.End()
			hooks := otel.KafkaHooks(provider, observabilitymetric.NewMeterProvider(), "workers")
			for _, hook := range hooks {
				if h, ok := hook.(kgo.HookFetchRecordBuffered); ok {
					h.OnFetchRecordBuffered(record)
				}
			}
			receive := trace.SpanContextFromContext(record.Context)
			for _, hook := range hooks {
				if h, ok := hook.(kgo.HookFetchRecordUnbuffered); ok {
					h.OnFetchRecordUnbuffered(record, false)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			tracer := otel.NewTracer(provider, "test")
			handler := middleware.Chain(func(ctx context.Context, _ *kgo.Record) error {
				current := trace.SpanContextFromContext(ctx)
				if current.TraceID() != receive.TraceID() || current.SpanID() == receive.SpanID() {
					t.Fatal("lost receive trace")
				}
				cancel()
				return ctx.Err()
			}, contextmw.New(tracer, contextmw.Config{HandlingTimeout: time.Second}).Handler(), New(tracer, logger.NoopLogger{}, contracts.NoopMeter{}).Handler("test.consume"), recoverymw.New(logger.NoopLogger{}).Handler())
			if err := handler(ctx, record); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			spans := exporter.GetSpans()
			processing := spans[len(spans)-1]
			if processing.Parent.SpanID() != receive.SpanID() || processing.Status.Code != codes.Error {
				t.Fatal("wrong processing parent or status")
			}
			sameTrace := receive.TraceID() == parent.SpanContext().TraceID()
			if sameTrace != withHeaders {
				t.Fatal("wrong receive trace origin")
			}
		})
	}
}
