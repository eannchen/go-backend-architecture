package observabilitymw

import (
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Middleware coordinates tracing, metrics, and access logging around each request.
type Middleware struct {
	tracing   *Tracing
	metrics   *RequestMetrics
	accessLog *AccessLog
}

// New creates HTTP observability middleware.
func New(tracer observability.Tracer, log logger.Logger, meter observability.Meter) *Middleware {
	return &Middleware{
		tracing:   NewTracing(tracer),
		metrics:   NewRequestMetrics(meter),
		accessLog: NewAccessLog(log),
	}
}

// Handler builds the single request lifecycle owned by observability.
func (m *Middleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			request := newRequestInfo(c)
			ctx, span := m.tracing.Start(c.Request().Context(), request)
			c.SetRequest(c.Request().WithContext(ctx))
			started := time.Now()

			handlerErr := next(c)
			outcome := newRequestOutcome(c, request, time.Since(started), handlerErr)
			completionCtx := c.Request().Context()
			m.metrics.Record(completionCtx, outcome)
			m.accessLog.Record(completionCtx, outcome)
			m.tracing.Finish(span, outcome)
			return handlerErr
		}
	}
}
