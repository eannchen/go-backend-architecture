package app

import (
	"context"

	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
)

func logEmitterToLogSink(emitter observability.LogEmitter) logger.LogSinkFunc {
	return func(ctx context.Context, severity logger.Severity, message string, fields ...logger.Field) {
		obsFields := make([]observability.Field, 0, len(fields))
		for _, f := range fields {
			obsFields = append(obsFields, observability.FieldOf(f.Key, f.Value))
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
	return func(ctx context.Context) []logger.Field {
		fields := make([]logger.Field, 0, 3)
		if requestID := observability.RequestIDFromContext(ctx); requestID != "" {
			fields = append(fields, logger.FieldOf("request_id", requestID))
		}
		traceID, spanID := observability.TraceFromContext(ctx)
		if traceID != "" {
			fields = append(fields, logger.FieldOf("trace_id", traceID))
		}
		if spanID != "" {
			fields = append(fields, logger.FieldOf("span_id", spanID))
		}
		return fields
	}
}
