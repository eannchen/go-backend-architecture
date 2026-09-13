package contextmw

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"golang.org/x/net/http/httpguts"

	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Config controls server deadlines and request-ID interoperability policy.
type Config struct {
	Timeout   time.Duration
	RequestID RequestIDConfig
}

// RequestIDConfig separates accepting and returning an optional request ID.
type RequestIDConfig struct {
	IncomingHeaderKey string
	ResponseHeaderKey string
	RejectInvalid     bool
}

// RequestContextMiddleware enriches request context with request ID and timeout.
type RequestContextMiddleware struct {
	config      Config
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
func NewRequestContextMiddleware(config Config, responder httpresponse.Responder, opts ...Option) (*RequestContextMiddleware, error) {
	var err error
	config.RequestID.IncomingHeaderKey, err = normalizeHeaderKey(config.RequestID.IncomingHeaderKey)
	if err != nil {
		return nil, fmt.Errorf("normalize incoming request ID header: %w", err)
	}
	config.RequestID.ResponseHeaderKey, err = normalizeHeaderKey(config.RequestID.ResponseHeaderKey)
	if err != nil {
		return nil, fmt.Errorf("normalize response request ID header: %w", err)
	}
	if responder == nil {
		responder = httpresponse.NewResponder()
	}
	m := &RequestContextMiddleware{
		config:    config,
		responder: responder,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// Handler builds the Echo middleware function for request context propagation.
func (m *RequestContextMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			reqCtx := req.Context()

			requestID, err := incomingRequestID(req, m.config.RequestID.IncomingHeaderKey)
			if err != nil && m.config.RequestID.RejectInvalid {
				headerKey := m.config.RequestID.IncomingHeaderKey
				return m.responder.Error(c,
					err,
					httpresponse.CodeInvalidRequestID,
					fmt.Sprintf("%s must be one value of 1-128 characters from [a-zA-Z0-9._-]", headerKey),
				)
			}
			if err == nil && requestID != "" {
				reqCtx = observability.WithRequestID(reqCtx, requestID)
				if key := m.config.RequestID.ResponseHeaderKey; key != "" {
					c.Response().Header().Set(key, requestID)
				}
			}

			if m.config.Timeout > 0 && (m.skipTimeout == nil || !m.skipTimeout(c)) {
				var cancel context.CancelFunc
				reqCtx, cancel = context.WithTimeout(reqCtx, m.config.Timeout)
				defer cancel()
			}

			// Echo stores a mutable request pointer, so replacing it makes this derived
			// context available both downstream and to outer middleware after it returns.
			c.SetRequest(req.WithContext(reqCtx))
			return next(c)
		}
	}
}

func incomingRequestID(req *http.Request, headerKey string) (string, error) {
	if headerKey == "" {
		return "", nil
	}
	values := req.Header.Values(headerKey)
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		return "", nil
	}
	if len(values) != 1 || !observability.IsValidRequestID(values[0]) {
		return "", fmt.Errorf("invalid %s header: %q", headerKey, values)
	}
	return values[0], nil
}

func normalizeHeaderKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil
	}
	if !httpguts.ValidHeaderFieldName(key) {
		return "", fmt.Errorf("invalid HTTP header name %q", key)
	}
	return http.CanonicalHeaderKey(key), nil
}
