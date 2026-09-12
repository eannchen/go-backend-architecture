package runtime

import (
	"context"
	"maps"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func logEmitterToLogSink(emitter observability.LogEmitter) logger.LogSinkFunc {
	return func(ctx context.Context, severity logger.Severity, message string, optionalFields ...logger.Fields) {
		fields := logger.OptionalFields(optionalFields...)
		obsFields := make(observability.Fields, len(fields))
		if len(fields) > 0 {
			maps.Copy(obsFields, observability.Fields(fields))
		}
		emitter.Emit(ctx, toObservabilitySeverity(severity), message, obsFields)
	}
}

func toObservabilitySeverity(s logger.Severity) observability.Severity {
	switch s {
	case logger.SeverityDebug:
		return observability.SeverityDebug
	case logger.SeverityWarn:
		return observability.SeverityWarn
	case logger.SeverityError:
		return observability.SeverityError
	default:
		return observability.SeverityInfo
	}
}

func contextFieldsProvider(tracer observability.Tracer) logger.ContextFieldsProviderFunc {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return func(ctx context.Context) logger.Fields {
		fields := make(logger.Fields)
		if id := observability.RequestIDFromContext(ctx); id != "" {
			fields["request.id"] = id
		}
		if traceContext, ok := tracer.TraceContext(ctx); ok {
			fields["trace.id"] = traceContext.TraceID
			fields["span.id"] = traceContext.SpanID
		}
		if len(fields) == 0 {
			return nil
		}
		return fields
	}
}
