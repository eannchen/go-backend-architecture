package observability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"vocynex-api/internal/infra/config"
)

// telemetryRuntime is a small lifecycle manager for OTel providers.
// Pattern used: Facade + Lifecycle Manager.
//
// Why this exists:
// - Setup() builds and installs trace/log providers.
// - Shutdown() flushes and closes providers in a controlled order.
// - Callers only need one ShutdownFunc.
type telemetryRuntime struct {
	traceProvider  *sdktrace.TracerProvider
	loggerProvider *sdklog.LoggerProvider
	logEmitter     LogEmitter
}

type otelLogEmitter struct {
	logger otellog.Logger
}

func logScopeName(serviceName string) string {
	return serviceName + "/logger"
}

func (e *otelLogEmitter) Emit(ctx context.Context, severity Severity, message string, attrs ...Field) {
	sev := toOTelSeverity(severity)
	if !e.logger.Enabled(ctx, otellog.EnabledParameters{Severity: sev}) {
		return
	}

	now := time.Now()
	var record otellog.Record
	record.SetTimestamp(now)
	record.SetObservedTimestamp(now)
	record.SetSeverity(sev)
	record.SetSeverityText(severity.String())
	record.SetBody(otellog.StringValue(message))
	record.AddAttributes(toLogAttributes(attrs...)...)

	e.logger.Emit(ctx, record)
}

type noopLogEmitter struct{}

func (noopLogEmitter) Emit(context.Context, Severity, string, ...Field) {}

func Setup(ctx context.Context, cfg config.OTelConfig, serviceName, appEnv string) (ShutdownFunc, LogEmitter, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, noopLogEmitter{}, nil
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
		return nil, nil, err
	}

	traceExporter, err := otlptracehttp.New(ctx, traceOptions...)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	logEmitter := &otelLogEmitter{logger: loggerProvider.Logger(logScopeName(serviceName))}

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

	runtime := &telemetryRuntime{
		traceProvider:  tracerProvider,
		loggerProvider: loggerProvider,
		logEmitter:     logEmitter,
	}
	return runtime.Shutdown, runtime.logEmitter, nil
}

// Shutdown flushes and closes OTel providers.
func (r *telemetryRuntime) Shutdown(ctx context.Context) error {
	return errors.Join(
		r.loggerProvider.Shutdown(ctx),
		r.traceProvider.Shutdown(ctx),
	)
}

func toOTelSeverity(level Severity) otellog.Severity {
	switch level {
	case SeverityDebug:
		return otellog.SeverityDebug
	case SeverityWarn:
		return otellog.SeverityWarn
	case SeverityError:
		return otellog.SeverityError
	default:
		return otellog.SeverityInfo
	}
}

func toLogAttributes(attrs ...Field) []otellog.KeyValue {
	out := make([]otellog.KeyValue, 0, len(attrs))
	for _, field := range attrs {
		out = append(out, otellog.KeyValue{
			Key:   field.Key,
			Value: toLogValue(field.Value),
		})
	}
	return out
}

func toLogValue(v any) otellog.Value {
	switch t := v.(type) {
	case string:
		return otellog.StringValue(t)
	case bool:
		return otellog.BoolValue(t)
	case int:
		return otellog.IntValue(t)
	case int32:
		return otellog.Int64Value(int64(t))
	case int64:
		return otellog.Int64Value(t)
	case uint:
		return otellog.Int64Value(int64(t))
	case uint32:
		return otellog.Int64Value(int64(t))
	case uint64:
		return otellog.Int64Value(int64(t))
	case float32:
		return otellog.Float64Value(float64(t))
	case float64:
		return otellog.Float64Value(t)
	case time.Time:
		return otellog.StringValue(t.Format(time.RFC3339Nano))
	case error:
		return otellog.StringValue(t.Error())
	default:
		return otellog.StringValue(fmt.Sprintf("%v", v))
	}
}
