package app

import (
	"context"

	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
)

func newObservabilityLogSink(emitter observability.LogEmitter) logger.LogSinkFunc {
	return func(ctx context.Context, severity logger.Severity, message string, fields ...logger.Field) {
		attrs := make([]observability.Field, 0, len(fields))
		for _, f := range fields {
			attrs = append(attrs, observability.FieldOf(f.Key, f.Value))
		}
		emitter.Emit(ctx, toObservabilitySeverity(severity), message, attrs...)
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
