package observabilitymw

import (
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// AccessLogMiddleware writes a single access log line per completed request.
type AccessLogMiddleware struct {
	log logger.Logger
}

// NewAccessLogMiddleware creates request access-log middleware.
func NewAccessLogMiddleware(log logger.Logger) *AccessLogMiddleware {
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &AccessLogMiddleware{log: log}
}

// Handler builds the Echo middleware function for request logs.
// Fields like request.id, trace.id, and span.id are injected automatically
// by the logger's ContextFieldsProvider; do not add them here.
func (m *AccessLogMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			handlerErr := next(c)
			duration := time.Since(start)

			req := c.Request()
			ctx := req.Context()
			route := c.Path()
			if route == "" {
				route = req.URL.Path
			}

			_, status := echo.ResolveResponseStatus(c.Response(), handlerErr)

			fields := logger.FromPairs(
				keyHTTPRequestMethod, req.Method,
				keyHTTPRoute, route,
				keyURLPath, req.URL.Path,
				keyHTTPResponseStatus, status,
				keyDurationMS, duration.Milliseconds(),
			)

			originalErr := httpcontext.Error(c)
			errorDetails := httpcontext.ErrorDetails(c)
			transportCode, transportMsg := httpcontext.TransportError(c)
			if originalErr != nil {
				fields[keyError] = originalErr.Error()
				fields[keyErrorChain] = observability.ErrorCauseChain(originalErr)
			}
			if len(errorDetails) > 0 {
				fields[keyErrorDetails] = errorDetails.String()
			}
			if transportCode != "" {
				fields[keyTransportCode] = transportCode
				fields[keyTransportMessage] = transportMsg
			}

			if status >= 500 {
				m.log.ErrorNoStack(ctx, "request completed", originalErr, fields)
				return handlerErr
			}

			m.log.Info(ctx, "request completed", fields)
			return handlerErr
		}
	}
}
