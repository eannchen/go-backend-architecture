package observabilitymw

import (
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Middleware composes tracing and request logging in a fixed order.
type Middleware struct {
	trace *TraceMiddleware
	log   *AccessLogMiddleware
}

// New creates fixed-order tracing and access-log middleware composition.
func New(tracer observability.Tracer, log logger.Logger) *Middleware {
	meta := httpcontext.NewContextMeta()
	return &Middleware{
		trace: NewTraceMiddleware(tracer, meta),
		log:   NewAccessLogMiddleware(log, meta),
	}
}

// Handler builds the Echo middleware function for observability composition.
func (m *Middleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return m.trace.Handler()(m.log.Handler()(next))
	}
}
