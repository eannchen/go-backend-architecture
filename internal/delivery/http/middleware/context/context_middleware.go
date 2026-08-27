package contextmw

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

const (
	requestIDHeader = "X-Request-ID"
)

// RequestContextMiddleware enriches request context with request ID and timeout.
type RequestContextMiddleware struct {
	timeout     time.Duration
	responder   httpresponse.Responder
	skipTimeout func(c *echo.Context) bool
}

// Option configures a RequestContextMiddleware.
type Option func(*RequestContextMiddleware)

// WithTimeoutSkipper exempts matching requests from the per-request deadline.
// SSE streams are bounded by their own timeout, not the short
// per-request deadline, which would otherwise cut them and trigger client
// reconnects.
func WithTimeoutSkipper(skip func(c *echo.Context) bool) Option {
	return func(m *RequestContextMiddleware) { m.skipTimeout = skip }
}

// NewRequestContextMiddleware creates request context middleware with optional timeout.
func NewRequestContextMiddleware(timeout time.Duration, responder httpresponse.Responder, opts ...Option) *RequestContextMiddleware {
	if responder == nil {
		responder = httpresponse.NewResponder()
	}
	m := &RequestContextMiddleware{
		timeout:   timeout,
		responder: responder,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Handler builds the Echo middleware function for request context propagation.
func (m *RequestContextMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			reqCtx := req.Context()

			requestID := req.Header.Get(requestIDHeader)
			switch {
			case requestID == "":
				var err error
				requestID, err = observability.GenerateRequestID()
				if err != nil {
					return m.responder.Error(c, err, httpresponse.Code(apperr.CodeInternal), "internal server error")
				}
			case !observability.IsValidRequestID(requestID):
				return m.responder.Error(c,
					fmt.Errorf("invalid X-Request-ID header: %q", requestID),
					httpresponse.CodeInvalidRequestID,
					"X-Request-ID must be 1-128 characters of [a-zA-Z0-9._-]",
				)
			}

			reqCtx = observability.WithRequestID(reqCtx, requestID)
			c.Response().Header().Set(requestIDHeader, requestID)

			if m.timeout > 0 && (m.skipTimeout == nil || !m.skipTimeout(c)) {
				var cancel context.CancelFunc
				reqCtx, cancel = context.WithTimeout(reqCtx, m.timeout)
				defer cancel()
			}

			c.SetRequest(req.WithContext(reqCtx))
			return next(c)
		}
	}
}
