package otel

import (
	"context"
	"errors"
	"testing"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type recordingLogExporter struct {
	records []sdklog.Record
}

func (e *recordingLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	for i := range records {
		e.records = append(e.records, records[i].Clone())
	}
	return nil
}

func (*recordingLogExporter) Shutdown(context.Context) error   { return nil }
func (*recordingLogExporter) ForceFlush(context.Context) error { return nil }

func TestOtelLogEmitter_EmitsStructuredRecord(t *testing.T) {
	exporter := &recordingLogExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	emitter := NewOtelLogEmitter(provider, "accounts-api")

	emitter.Emit(context.Background(), observability.SeverityWarn, "cache unavailable", observability.FromPairs(
		"user.id", int64(42),
		"retryable", true,
	))

	if len(exporter.records) != 1 {
		t.Fatalf("exported records = %d, want 1", len(exporter.records))
	}
	record := exporter.records[0]
	if record.Severity() != otellog.SeverityWarn || record.SeverityText() != "warn" {
		t.Fatalf("severity = %v %q, want warn", record.Severity(), record.SeverityText())
	}
	if got := record.Body().AsString(); got != "cache unavailable" {
		t.Fatalf("body = %q, want cache unavailable", got)
	}
	attributes := logAttributes(record)
	if attributes["user.id"].AsInt64() != 42 || !attributes["retryable"].AsBool() {
		t.Fatalf("unexpected attributes: %+v", attributes)
	}
	if record.Timestamp().IsZero() || record.ObservedTimestamp().IsZero() {
		t.Fatal("expected record timestamps to be populated")
	}
}

func TestToLogValue(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 30, 0, 123, time.UTC)
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "string", in: "value", want: "value"},
		{name: "integer", in: int64(42), want: "42"},
		{name: "boolean", in: true, want: "true"},
		{name: "float", in: 1.5, want: "1.5"},
		{name: "time", in: now, want: now.Format(time.RFC3339Nano)},
		{name: "error", in: errors.New("failed"), want: "failed"},
		{name: "fallback", in: struct{ ID int }{ID: 7}, want: "{7}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toLogValue(tt.in).String(); got != tt.want {
				t.Fatalf("converted value = %q, want %q", got, tt.want)
			}
		})
	}
}

func logAttributes(record sdklog.Record) map[string]otellog.Value {
	out := make(map[string]otellog.Value, record.AttributesLen())
	record.WalkAttributes(func(attribute otellog.KeyValue) bool {
		out[attribute.Key] = attribute.Value
		return true
	})
	return out
}
