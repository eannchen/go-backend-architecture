package app

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
