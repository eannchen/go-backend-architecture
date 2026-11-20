package otel

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

// Config controls OTLP-over-HTTP export and trace sampling. Applications supply
// complete endpoint URLs; environment loading belongs to the application.
type Config struct {
	// ExportEnabled enables collector traffic. When false, trace IDs and
	// propagation remain active, but no signals are sent to a collector.
	ExportEnabled   bool
	TracesEndpoint  string
	LogsEndpoint    string
	MetricsEndpoint string
	// Insecure permits export without TLS, typically for a local collector.
	Insecure bool
	// TraceSamplingRatio is the fraction of new root traces to record, from
	// 0 to 1. Child spans follow their parent's sampling decision.
	TraceSamplingRatio float64
}

// The runtime owns provider shutdown as well as instrumentation. OTel's API
// interfaces expose instrumentation only, so require Shutdown here too.
type traceProvider interface {
	trace.TracerProvider
	Shutdown(context.Context) error
}

type meterProvider interface {
	metric.MeterProvider
	Shutdown(context.Context) error
}

type logProvider interface {
	otellog.LoggerProvider
	Shutdown(context.Context) error
}

// Runtime owns OTel providers and supplies both application contracts and SDK
// instrumentation built from those same providers.
type Runtime struct {
	traceProvider  traceProvider
	loggerProvider logProvider
	meterProvider  meterProvider
	logEmitter     observability.LogEmitter
	tracer         observability.Tracer
	meter          observability.Meter
}

// TracerProvider and MeterProvider let client SDKs use the same telemetry
// providers as the application. The runtime remains responsible for shutdown.
func (r *Runtime) TracerProvider() trace.TracerProvider { return r.traceProvider }
func (r *Runtime) MeterProvider() metric.MeterProvider  { return r.meterProvider }

func (r *Runtime) LogEmitter() observability.LogEmitter {
	return r.logEmitter
}

func (r *Runtime) Tracer() observability.Tracer {
	return r.tracer
}

func (r *Runtime) Meter() observability.Meter {
	return r.meter
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	return errors.Join(
		shutdownError("shutdown meter provider", r.meterProvider.Shutdown(ctx)),
		shutdownError("shutdown logger provider", r.loggerProvider.Shutdown(ctx)),
		shutdownError("shutdown tracer provider", r.traceProvider.Shutdown(ctx)),
	)
}

// Setup creates providers without changing OTel's global configuration. The
// caller owns Runtime.Shutdown and must stop instrumented clients first.
func Setup(ctx context.Context, cfg Config, serviceName, appEnv string) (*Runtime, error) {
	if !(cfg.TraceSamplingRatio >= 0 && cfg.TraceSamplingRatio <= 1) {
		return nil, fmt.Errorf("trace sampling ratio must be between 0 and 1")
	}
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("deployment.environment", appEnv),
		),
	)
	if err != nil {
		return nil, err
	}

	if !cfg.ExportEnabled {
		// Keep the SDK and propagator active even when this process must not contact
		// a collector. Spans still carry native IDs for stdout correlation and
		// distributed context can continue through this service.
		return newRuntime(
			sdktrace.NewTracerProvider(
				sdktrace.WithResource(res),
				sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSamplingRatio))),
			),
			sdklog.NewLoggerProvider(sdklog.WithResource(res)),
			sdkmetric.NewMeterProvider(sdkmetric.WithResource(res)),
			serviceName,
		), nil
	}

	traceOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(cfg.TracesEndpoint),
	}
	logOptions := []otlploghttp.Option{
		otlploghttp.WithEndpointURL(cfg.LogsEndpoint),
	}
	metricOptions := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpointURL(cfg.MetricsEndpoint),
	}
	if cfg.Insecure {
		traceOptions = append(traceOptions, otlptracehttp.WithInsecure())
		logOptions = append(logOptions, otlploghttp.WithInsecure())
		metricOptions = append(metricOptions, otlpmetrichttp.WithInsecure())
	}

	traceExporter, err := otlptracehttp.New(ctx, traceOptions...)
	if err != nil {
		return nil, err
	}
	// Completed spans are exported as structured OTLP records containing trace,
	// span, and parent IDs. A backend such as HyperDX rebuilds the distributed
	// trace from those relationships; it does not need the raw traceparent header.
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		// Sampler controls "which traces are kept/exported".
		// - TraceIDRatioBased(r): for NEW root traces, keep about r (0.0~1.0).
		// - ParentBased(...): if this request already has a parent span, follow
		//   the parent's sampling decision for trace consistency across services.
		// Example: r=0.1 means ~10% of new traces are sampled.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSamplingRatio))),
	)

	logExporter, err := otlploghttp.New(ctx, logOptions...)
	if err != nil {
		return nil, errors.Join(
			err,
			shutdownError("shutdown tracer provider after log exporter init failure", tracerProvider.Shutdown(ctx)),
		)
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)

	metricExporter, err := otlpmetrichttp.New(ctx, metricOptions...)
	if err != nil {
		return nil, errors.Join(
			err,
			shutdownError("shutdown logger provider after metric exporter init failure", loggerProvider.Shutdown(ctx)),
			shutdownError("shutdown tracer provider after metric exporter init failure", tracerProvider.Shutdown(ctx)),
		)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)

	return newRuntime(tracerProvider, loggerProvider, meterProvider, serviceName), nil
}

func newRuntime(
	tracerProvider traceProvider,
	loggerProvider logProvider,
	meterProvider meterProvider,
	serviceName string,
) *Runtime {
	return &Runtime{
		traceProvider:  tracerProvider,
		loggerProvider: loggerProvider,
		meterProvider:  meterProvider,
		logEmitter:     NewOtelLogEmitter(loggerProvider, serviceName),
		tracer:         NewTracer(tracerProvider, serviceName),
		meter:          NewMeter(meterProvider, serviceName),
	}
}

// shutdownError preserves provider failures without turning successful cleanup
// into an error when independent shutdown results are joined.
func shutdownError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
