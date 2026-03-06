package otel

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/observability"
)

// otelRuntime is a lifecycle manager for OTel providers.
// Pattern used: Facade + Lifecycle Manager.
type otelRuntime struct {
	traceProvider  *sdktrace.TracerProvider
	loggerProvider *sdklog.LoggerProvider
	logEmitter     observability.LogEmitter
	tracer         observability.Tracer
}

func (r *otelRuntime) LogEmitter() observability.LogEmitter {
	return r.logEmitter
}

func (r *otelRuntime) Tracer() observability.Tracer {
	return r.tracer
}

func (r *otelRuntime) Shutdown(ctx context.Context) error {
	return errors.Join(
		r.loggerProvider.Shutdown(ctx),
		r.traceProvider.Shutdown(ctx),
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
	if cfg.Insecure {
		traceOptions = append(traceOptions, otlptracehttp.WithInsecure())
		logOptions = append(logOptions, otlploghttp.WithInsecure())
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
		_ = tracerProvider.Shutdown(ctx)
		return nil, err
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	logEmitter := NewOtelLogEmitter(loggerProvider, serviceName)

	otel.SetTracerProvider(tracerProvider)
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
		logEmitter:     logEmitter,
		tracer:         NewTracer(serviceName),
	}
	return runtime, nil
}
