package otel

import (
	"context"
	"errors"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/observability"
	recoverymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	observabilitymetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestKafkaHooksTraceAndMetrics verifies the configured hooks use runtime
// providers, propagate publish/receive context, omit keys, and emit client metrics.
func TestKafkaHooksTraceAndMetrics(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	reader := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(cleanupCtx); err != nil {
			t.Error(err)
		}
		if err := meter.Shutdown(cleanupCtx); err != nil {
			t.Error(err)
		}
	})
	runtime := &Runtime{traceProvider: provider, meterProvider: meter}
	hooks := KafkaHooks(runtime.TracerProvider(), runtime.MeterProvider(), "workers")
	ctx, root := provider.Tracer("test").Start(t.Context(), "request")
	record := &kgo.Record{Context: ctx, Topic: "notifications", Partition: 0, Offset: 7, Key: []byte("secret-recipient"), Value: []byte("secret-body")}
	for _, h := range hooks {
		if hook, ok := h.(kgo.HookProduceRecordBuffered); ok {
			hook.OnProduceRecordBuffered(record)
		}
	}
	publish := trace.SpanContextFromContext(record.Context)
	if publish.TraceID() != root.SpanContext().TraceID() || publish.SpanID() == root.SpanContext().SpanID() {
		t.Fatal("producer did not create a child span")
	}
	for _, h := range hooks {
		if hook, ok := h.(kgo.HookProduceRecordUnbuffered); ok {
			hook.OnProduceRecordUnbuffered(record, nil)
		}
	}
	received := &kgo.Record{Topic: record.Topic, Partition: 0, Offset: 7, Headers: record.Headers, Key: record.Key, Value: record.Value}
	for _, h := range hooks {
		if hook, ok := h.(kgo.HookFetchRecordBuffered); ok {
			hook.OnFetchRecordBuffered(received)
		}
	}
	receive := trace.SpanContextFromContext(received.Context)
	if receive.TraceID() != publish.TraceID() {
		t.Fatal("consumer lost producer trace")
	}
	for _, h := range hooks {
		if hook, ok := h.(kgo.HookFetchRecordUnbuffered); ok {
			hook.OnFetchRecordUnbuffered(received, false)
		}
		if hook, ok := h.(kgo.HookBrokerWrite); ok {
			hook.OnBrokerWrite(kgo.BrokerMetadata{NodeID: 1}, 0, 123, 0, 0, nil)
		}
	}
	root.End()
	spans := exporter.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("spans=%d", len(spans))
	}
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == "messaging.kafka.message.key" || attr.Value.AsString() == "secret-recipient" || attr.Value.AsString() == "secret-body" {
				t.Fatalf("sensitive span attribute %v", attr.Key)
			}
		}
	}
	if spans[1].Parent.SpanID() != publish.SpanID() {
		t.Fatal("receive span is not a child of publish")
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &data); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == "messaging.kafka.write_bytes" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("missing Kafka byte metric: %+v", data)
	}
}

// TestKafkaProcessingContinuesReceiveSpan verifies the real tracing adapter
// parents processing to kotel's ended receive span and keeps worker cancellation.
func TestKafkaProcessingContinuesReceiveSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otelapi.GetTracerProvider()
	otelapi.SetTracerProvider(provider)
	t.Cleanup(func() {
		otelapi.SetTracerProvider(previous)
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
				carrier := messageTestCarrier{}
				propagation.TraceContext{}.Inject(parentCtx, carrier)
				for key, value := range carrier {
					record.Headers = append(record.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
				}
			}
			parent.End()
			hooks := KafkaHooks(provider, observabilitymetric.NewMeterProvider(), "workers")
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
			tracer := NewTracer("test")
			handler := middleware.Chain(func(ctx context.Context, _ *kgo.Record) error {
				current := trace.SpanContextFromContext(ctx)
				if current.TraceID() != receive.TraceID() || current.SpanID() == receive.SpanID() {
					t.Fatal("lost receive trace")
				}
				cancel()
				return ctx.Err()
			}, contextmw.New(tracer, contextmw.Config{HandlingTimeout: time.Second}).Handler(), observabilitymw.New(tracer, nil, nil).Handler("test.consume"), recoverymw.New(nil).Handler())
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

type messageTestCarrier map[string]string

func (c messageTestCarrier) Get(key string) string { return c[key] }
func (c messageTestCarrier) Set(key, value string) { c[key] = value }
func (c messageTestCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}
