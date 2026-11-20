package otel

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	apiotel "go.opentelemetry.io/otel"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	meternoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// TestSetupLeavesGlobalsUnchanged ensures constructing a library runtime cannot
// replace telemetry already configured by its host application.
func TestSetupLeavesGlobalsUnchanged(t *testing.T) {
	restoreGlobalsAfterTest(t)
	traces := tracenoop.NewTracerProvider()
	meters := meternoop.NewMeterProvider()
	propagator := &propagation.TraceContext{}
	apiotel.SetTracerProvider(traces)
	apiotel.SetMeterProvider(meters)
	apiotel.SetTextMapPropagator(propagator)

	runtime, err := Setup(t.Context(), Config{}, "library", "test")
	if err != nil {
		t.Fatal(err)
	}
	shutdownRuntimeAfterTest(t, runtime)
	if apiotel.GetTracerProvider() != traces || apiotel.GetMeterProvider() != meters || apiotel.GetTextMapPropagator() != propagator {
		t.Fatal("Setup replaced the host application's global telemetry")
	}
}

// TestRuntimeTracersKeepOwnProviders protects runtime isolation when another
// runtime is created and an external component changes the global provider.
func TestRuntimeTracersKeepOwnProviders(t *testing.T) {
	restoreGlobalsAfterTest(t)
	recorderA := tracetest.NewSpanRecorder()
	providerA := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorderA))
	runtimeA := newRuntime(providerA, sdklog.NewLoggerProvider(), sdkmetric.NewMeterProvider(), "service-a")
	shutdownRuntimeAfterTest(t, runtimeA)
	recorderB := tracetest.NewSpanRecorder()
	providerB := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorderB))
	runtimeB := newRuntime(providerB, sdklog.NewLoggerProvider(), sdkmetric.NewMeterProvider(), "service-b")
	shutdownRuntimeAfterTest(t, runtimeB)
	apiotel.SetTracerProvider(providerB)

	_, spanA := runtimeA.Tracer().Start(t.Context(), "test", "operation-a")
	spanA.Finish(nil)
	_, spanB := runtimeB.Tracer().Start(t.Context(), "test", "operation-b")
	spanB.Finish(nil)

	spansA, spansB := recorderA.Ended(), recorderB.Ended()
	if len(spansA) != 1 || spansA[0].Name() != "operation-a" {
		t.Fatalf("runtime A's provider recorded the wrong spans: %v", spansA)
	}
	if len(spansB) != 1 || spansB[0].Name() != "operation-b" {
		t.Fatalf("runtime B's provider recorded the wrong spans: %v", spansB)
	}
}

// TestSetupRejectsInvalidSamplingRatio protects callers that supply configuration
// directly, without the template's environment loader.
func TestSetupRejectsInvalidSamplingRatio(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ratio float64
	}{
		{name: "negative", ratio: -0.1},
		{name: "above one", ratio: 1.1},
		{name: "not a number", ratio: math.NaN()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := Setup(t.Context(), Config{TraceSamplingRatio: tt.ratio}, "test", "test")
			if err == nil || runtime != nil {
				t.Fatalf("Setup() = %v, %v; want an error and no runtime", runtime, err)
			}
		})
	}
}

// TestSetup_ExportDisabledKeepsTraceContextActive checks disabled export still leaves trace context propagation available.
func TestSetup_ExportDisabledKeepsTraceContextActive(t *testing.T) {
	runtime, err := Setup(context.Background(), Config{
		ExportEnabled: false,
	}, "accounts-api", "test")
	if err != nil {
		t.Fatalf("setup with export disabled: %v", err)
	}
	shutdownRuntimeAfterTest(t, runtime)

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

func restoreGlobalsAfterTest(t *testing.T) {
	t.Helper()
	traces := apiotel.GetTracerProvider()
	meters := apiotel.GetMeterProvider()
	propagator := apiotel.GetTextMapPropagator()
	t.Cleanup(func() {
		apiotel.SetTracerProvider(traces)
		apiotel.SetMeterProvider(meters)
		apiotel.SetTextMapPropagator(propagator)
	})
}

func shutdownRuntimeAfterTest(t *testing.T, runtime *Runtime) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
}
