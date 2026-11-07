package observability

import (
	"context"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Middleware coordinates traces, metrics, and completion logs for record handling.
// Commits and acknowledgments happen afterward in the worker.
type Middleware struct {
	tracing   *Tracing
	metrics   *ProcessingMetrics
	accessLog *AccessLog
}

func New(tracer observability.Tracer, log logger.Logger, meter observability.Meter) *Middleware {
	return &Middleware{tracing: NewTracing(tracer), metrics: NewProcessingMetrics(meter), accessLog: NewAccessLog(log)}
}

// Handler wraps Recovery so recovered panics become visible processing outcomes.
// operation is a fixed app-selected name, never a value taken from a payload.
func (m *Middleware) Handler(operation string) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, record *kgo.Record) error {
			ctx, span := m.tracing.Start(ctx, operation, record)
			started := time.Now()
			err := ctx.Err()
			if err == nil {
				err = next(ctx, record)
			}
			// Context middleware owns the deadline. Observe cancellation before recording
			// the outcome, even if the handler returns nil after that deadline.
			if err == nil {
				err = ctx.Err()
			}
			outcome := newProcessingOutcome(operation, record, time.Since(started), err)
			m.metrics.Record(ctx, outcome)
			m.accessLog.Record(ctx, outcome)
			m.tracing.Finish(span, outcome)
			return err
		}
	}
}
