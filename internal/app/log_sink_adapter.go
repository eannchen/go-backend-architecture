package app

import (
	"context"

	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
)

func newObservabilityLogSink(emitter observability.LogEmitter) logger.LogSinkFunc {
	return func(ctx context.Context, severityText, message string, fields ...logger.Field) {
		attrs := make([]observability.KV, 0, len(fields))
		for _, f := range fields {
			attrs = append(attrs, observability.KV{
				Key:   f.Key,
				Value: f.Value,
			})
		}
		emitter.Emit(ctx, severityText, message, attrs...)
	}
}
