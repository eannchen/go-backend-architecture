package otel

import (
	"context"
	"errors"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"slices"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
)

// TestSetup_ExportDisabledKeepsTraceContextActive checks disabled export still leaves trace context propagation available.
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

// TestRuntimeOwnsProviderLifecycle protects explicit provider identity and ensures
// every shutdown is attempted in order even when earlier providers fail.
func TestRuntimeOwnsProviderLifecycle(t *testing.T) {
	ctx := t.Context()
	meterErr := errors.New("meter shutdown failed")
	logErr := errors.New("log shutdown failed")
	traceErr := errors.New("trace shutdown failed")
	var order []string
	shutdown := func(name string, failure error) shutdownProvider {
		return shutdownProvider{ShutdownFunc: func(got context.Context) error {
			if got != ctx {
				t.Error("shutdown context was replaced")
			}
			order = append(order, name)
			return failure
		}}
	}
	tracing := &runtimeTraceProvider{shutdownProvider: shutdown("trace", traceErr)}
	logging := &runtimeLogProvider{shutdownProvider: shutdown("log", logErr)}
	metering := &runtimeMeterProvider{shutdownProvider: shutdown("meter", meterErr)}
	runtime := &Runtime{traceProvider: tracing, loggerProvider: logging, meterProvider: metering}
	if runtime.TracerProvider() != tracing || runtime.MeterProvider() != metering {
		t.Fatal("provider getters returned different instances")
	}
	err := runtime.Shutdown(ctx)
	for _, failure := range []error{meterErr, logErr, traceErr} {
		if !errors.Is(err, failure) {
			t.Fatalf("shutdown lost %v: %v", failure, err)
		}
	}
	if !slices.Equal(order, []string{"meter", "log", "trace"}) {
		t.Fatalf("shutdown order=%v", order)
	}
}

type shutdownProvider struct{ ShutdownFunc func(context.Context) error }

func (p shutdownProvider) Shutdown(ctx context.Context) error { return p.ShutdownFunc(ctx) }

// Embedded API interfaces supply the unused instrumentation methods. This test
// replaces provider shutdown only, so creating instruments would be unexpected.
type runtimeTraceProvider struct {
	trace.TracerProvider
	shutdownProvider
}
type runtimeLogProvider struct {
	otellog.LoggerProvider
	shutdownProvider
}
type runtimeMeterProvider struct {
	metric.MeterProvider
	shutdownProvider
}
