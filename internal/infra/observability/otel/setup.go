package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go-backend-architecture/internal/infra/config"
	"go-backend-architecture/internal/observability"
	"go-backend-architecture/internal/util/errutil"
)

// otelRuntime is a lifecycle manager for OTel providers.
// Pattern used: Facade + Lifecycle Manager.
type otelRuntime struct {
	traceProvider  *sdktrace.TracerProvider
	loggerProvider *sdklog.LoggerProvider
	meterProvider  *sdkmetric.MeterProvider
	logEmitter     observability.LogEmitter
	tracer         observability.Tracer
	meter          observability.Meter
}

func (r *otelRuntime) LogEmitter() observability.LogEmitter {
	return r.logEmitter
}

func (r *otelRuntime) Tracer() observability.Tracer {
	return r.tracer
}

func (r *otelRuntime) Meter() observability.Meter {
	return r.meter
}

func (r *otelRuntime) Shutdown(ctx context.Context) error {
	return errutil.Join(
		errutil.Step("shutdown meter provider", r.meterProvider.Shutdown(ctx)),
		errutil.Step("shutdown logger provider", r.loggerProvider.Shutdown(ctx)),
		errutil.Step("shutdown tracer provider", r.traceProvider.Shutdown(ctx)),
	)
}

func Setup(ctx context.Context, cfg config.OTelConfig, serviceName, appEnv string) (observability.Runtime, error) {
	if !cfg.Enabled {
		return observability.NoopRuntime{}, nil
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

	traceExporter, err := otlptracehttp.New(ctx, traceOptions...)
	if err != nil {
		return nil, err
	}
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
	logEmitter := NewOtelLogEmitter(loggerProvider, serviceName)

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

	runtime := &otelRuntime{
		traceProvider:  tracerProvider,
		loggerProvider: loggerProvider,
		meterProvider:  meterProvider,
		logEmitter:     logEmitter,
		tracer:         NewTracer(serviceName),
		meter:          NewMeter(meterProvider, serviceName),
	}
	return runtime, nil
}
