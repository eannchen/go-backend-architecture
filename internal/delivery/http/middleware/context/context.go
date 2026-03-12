package contextmw

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/labstack/echo/v5"

	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

const (
	requestIDHeader = "X-Request-ID"
	maxRequestIDLen = 128
)

// RequestContextMiddleware enriches request context with request ID and timeout.
type RequestContextMiddleware struct {
	timeout   time.Duration
	responder httpresponse.Responder
}

// NewRequestContextMiddleware creates request context middleware with optional timeout.
func NewRequestContextMiddleware(timeout time.Duration, responder httpresponse.Responder) *RequestContextMiddleware {
	if responder == nil {
		responder = httpresponse.NewResponder(nil)
	}
	return &RequestContextMiddleware{
		timeout:   timeout,
		responder: responder,
	}
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
				requestID = randomID()
			case !isValidRequestID(requestID):
				return m.responder.Error(c,
					fmt.Errorf("invalid X-Request-ID header: %q", requestID),
					httpresponse.CodeInvalidRequestID,
					"X-Request-ID must be 1-128 characters of [a-zA-Z0-9._-]",
				)
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

func isValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > maxRequestIDLen {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
