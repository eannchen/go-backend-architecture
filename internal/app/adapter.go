package app

import (
	"context"

	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
)

func logEmitterToLogSink(emitter observability.LogEmitter) logger.LogSinkFunc {
	return func(ctx context.Context, severity logger.Severity, message string, optionalFields ...logger.Fields) {
		fields := logger.OptionalFields(optionalFields...)
		obsFields := make([]observability.Field, 0, len(fields))
		for key, value := range fields {
			obsFields = append(obsFields, observability.FieldOf(key, value))
		}
		emitter.Emit(ctx, toObservabilitySeverity(severity), message, obsFields...)
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

func contextFieldsProvider() logger.ContextFieldsProviderFunc {
	return func(ctx context.Context) logger.Fields {
		fields := make(logger.Fields)
		if requestID := observability.RequestIDFromContext(ctx); requestID != "" {
			fields["request_id"] = requestID
		}
		traceID, spanID := observability.TraceFromContext(ctx)
		if traceID != "" {
			fields["trace_id"] = traceID
		}
		if spanID != "" {
			fields["span_id"] = spanID
		}
		if len(fields) == 0 {
			return nil
		}
		return fields
	}
}
