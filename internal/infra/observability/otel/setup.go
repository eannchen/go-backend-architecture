package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

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
	return errutil.Join(
		errutil.Step("shutdown meter provider", r.meterProvider.Shutdown(ctx)),
		errutil.Step("shutdown logger provider", r.loggerProvider.Shutdown(ctx)),
		errutil.Step("shutdown tracer provider", r.traceProvider.Shutdown(ctx)),
	)
}

func Setup(ctx context.Context, cfg config.OTelConfig, serviceName, appEnv string) (*Runtime, error) {
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
		return nil, errutil.Join(
			err,
			errutil.Step("shutdown tracer provider after log exporter init failure", tracerProvider.Shutdown(ctx)),
		)
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)

	metricExporter, err := otlpmetrichttp.New(ctx, metricOptions...)
	if err != nil {
		return nil, errutil.Join(
			err,
			errutil.Step("shutdown logger provider after metric exporter init failure", loggerProvider.Shutdown(ctx)),
			errutil.Step("shutdown tracer provider after metric exporter init failure", tracerProvider.Shutdown(ctx)),
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
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	// Propagator is the "format adapter" used to read/write trace context in
	// carriers like HTTP headers. This enables cross-service trace continuation.
	// - TraceContext: handles W3C trace headers (traceparent/tracestate).
	// - Baggage: handles W3C baggage key-values.
	// We set this globally so middleware/outbound clients use a consistent format.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Runtime{
		traceProvider:  tracerProvider,
		loggerProvider: loggerProvider,
		meterProvider:  meterProvider,
		logEmitter:     NewOtelLogEmitter(loggerProvider, serviceName),
		tracer:         NewTracer(serviceName),
		meter:          NewMeter(meterProvider, serviceName),
	}
}
