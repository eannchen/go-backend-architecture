package observabilitymw

import (
	"time"

	"github.com/labstack/echo/v5"

	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// AccessLogMiddleware writes a single access log line per completed request.
type AccessLogMiddleware struct {
	log  logger.Logger
	meta httpresponse.Meta
}

// NewAccessLogMiddleware creates access-log middleware with shared metadata.
func NewAccessLogMiddleware(log logger.Logger, meta httpresponse.Meta) *AccessLogMiddleware {
	if meta == nil {
		meta = httpresponse.NewContextMeta()
	}
	return &AccessLogMiddleware{
		log:  log,
		meta: meta,
	}
}

// Handler builds the Echo middleware function for request logs.
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

			requestID := observability.RequestIDFromContext(ctx)
			traceID, spanID := observability.TraceFromContext(ctx)
			_, status := echo.ResolveResponseStatus(c.Response(), handlerErr)

			fields := logger.FromPairs(
				keyHTTPRequestMethod, req.Method,
				keyHTTPRoute, route,
				keyURLPath, req.URL.Path,
				keyHTTPResponseStatus, status,
				keyRequestID, requestID,
				keyDurationMS, duration.Milliseconds(),
			)
			if traceID != "" {
				fields[keyTraceID] = traceID
			}
			if spanID != "" {
				fields[keySpanID] = spanID
			}

			originalErr := m.meta.GetError(c)
			errorDetails := m.meta.GetErrorDetails(c)
			transportCode, transportMsg := m.meta.GetTransportError(c)
			if originalErr != nil {
				fields[keyError] = originalErr.Error()
			}
			if len(errorDetails) > 0 {
				fields[keyErrorDetails] = errorDetails.String()
			}
			if transportCode != "" {
				fields[keyTransportCode] = transportCode
				fields[keyTransportMessage] = transportMsg
			}

			if status >= 500 {
				m.log.Error(ctx, "request completed", originalErr, fields)
				return handlerErr
			}

			m.log.Info(ctx, "request completed", fields)
			return handlerErr
		}
	}
}
