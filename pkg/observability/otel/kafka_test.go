package otel

import (
	"context"
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
