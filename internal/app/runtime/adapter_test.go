package runtime

import (
	"context"
	"reflect"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
)

func TestToObservabilitySeverity(t *testing.T) {
	tests := []struct {
		name string
		in   logger.Severity
		want observability.Severity
	}{
		{name: "debug", in: logger.SeverityDebug, want: observability.SeverityDebug},
		{name: "info", in: logger.SeverityInfo, want: observability.SeverityInfo},
		{name: "warn", in: logger.SeverityWarn, want: observability.SeverityWarn},
		{name: "error", in: logger.SeverityError, want: observability.SeverityError},
		{name: "unknown defaults to info", in: logger.Severity(255), want: observability.SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toObservabilitySeverity(tt.in); got != tt.want {
				t.Fatalf("toObservabilitySeverity(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestLogEmitterToLogSink(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request"), "request-1")
	emitter := &observabilitytest.LogEmitter{
		EmitFunc: func(context.Context, observability.Severity, string, ...observability.Fields) {},
	}
	sink := logEmitterToLogSink(emitter)
	fields := logger.Fields{"component": "api"}

	sink(ctx, logger.SeverityWarn, "request failed", fields)
	// The adapter owns the converted map, so later logger mutations cannot
	// change a record already handed to observability.
	fields["component"] = "worker"

	if len(emitter.EmitCalls) != 1 {
		t.Fatalf("Emit() calls = %d, want 1", len(emitter.EmitCalls))
	}
	got := emitter.EmitCalls[0]
	if got.Context != ctx || got.Severity != observability.SeverityWarn || got.Message != "request failed" {
		t.Fatalf("Emit() call = %+v, want forwarded context, severity, and message", got)
	}
	wantFields := observability.Fields{"component": "api"}
	if len(got.Fields) != 1 || !reflect.DeepEqual(got.Fields[0], wantFields) {
		t.Fatalf("Emit() fields = %#v, want %#v", got.Fields, wantFields)
	}
}

func TestContextFieldsProvider(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want logger.Fields
	}{
		{name: "empty context", ctx: context.Background()},
		{
			name: "request id",
			ctx:  observability.WithRequestID(context.Background(), "request-1"),
			want: logger.Fields{"request.id": "request-1"},
		},
		{
			name: "trace and span ids",
			ctx:  observability.WithTrace(context.Background(), "trace-1", "span-1"),
			want: logger.Fields{"trace.id": "trace-1", "span.id": "span-1"},
		},
		{
			name: "all correlation ids",
			ctx: observability.WithTrace(
				observability.WithRequestID(context.Background(), "request-1"),
				"trace-1",
				"span-1",
			),
			want: logger.Fields{"request.id": "request-1", "trace.id": "trace-1", "span.id": "span-1"},
		},
	}

	provider := contextFieldsProvider()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := provider(tt.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("context fields = %#v, want %#v", got, tt.want)
			}
		})
	}
}
