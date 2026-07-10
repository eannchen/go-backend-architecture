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
		responder = httpresponse.NewResponder(nil)
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
