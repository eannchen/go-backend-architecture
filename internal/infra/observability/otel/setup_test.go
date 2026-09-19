package otel

import (
	"context"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
)

func TestSetup_ExportDisabledKeepsTraceContextActive(t *testing.T) {
	runtime, err := Setup(context.Background(), config.OTelConfig{
		ExportEnabled: false,
	}, "accounts-api", "test")
	if err != nil {
		t.Fatalf("setup with export disabled: %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	ctx, span := runtime.Tracer().Start(context.Background(), "test", "operation")
	traceContext, ok := runtime.Tracer().TraceContext(ctx)
	span.Finish(nil)
	if !ok || traceContext.TraceID == "" || traceContext.SpanID == "" {
		t.Fatalf("trace context = %+v, %v; want valid IDs", traceContext, ok)
	}
}
