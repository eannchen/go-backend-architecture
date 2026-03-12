package contextmw

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

const requestIDHeader = "X-Request-ID"

// RequestContextMiddleware enriches request context with request ID and timeout.
type RequestContextMiddleware struct {
	timeout time.Duration
}

// NewRequestContextMiddleware creates request context middleware with optional timeout.
func NewRequestContextMiddleware(timeout time.Duration) *RequestContextMiddleware {
	return &RequestContextMiddleware{
		timeout: timeout,
	}
}

// Handler builds the Echo middleware function for request context propagation.
func (m *RequestContextMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			reqCtx := req.Context()

			requestID := req.Header.Get(requestIDHeader)
			if requestID == "" {
				requestID = randomID()
			}

			reqCtx = observability.WithRequestID(reqCtx, requestID)
			c.Response().Header().Set(requestIDHeader, requestID)

			if m.timeout > 0 {
				var cancel context.CancelFunc
				reqCtx, cancel = context.WithTimeout(reqCtx, m.timeout)
				defer cancel()
			}

			c.SetRequest(req.WithContext(reqCtx))
			return next(c)
		}
	}
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
