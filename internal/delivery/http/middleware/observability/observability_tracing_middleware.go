package observabilitymw

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// TraceMiddleware manages request span lifecycle and response/error trace attributes.
type TraceMiddleware struct {
	tracer observability.Tracer
}

// NewTraceMiddleware creates request tracing middleware.
func NewTraceMiddleware(tracer observability.Tracer) *TraceMiddleware {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return &TraceMiddleware{tracer: tracer}
}

// Handler builds the Echo middleware function for request tracing.
func (m *TraceMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := m.tracer.Extract(req.Context(), headerCarrier{Header: req.Header})

			route := c.Path()
			if route == "" {
				route = req.URL.Path
			}

			spanName := fmt.Sprintf("%s %s", req.Method, route)
			ctx, span := m.tracer.StartServer(
				ctx,
				"http",
				spanName,
				observability.FromPairs(
					keyHTTPRequestMethod, req.Method,
					keyHTTPRoute, route,
					keyURLPath, req.URL.Path,
				),
			)
			if traceID, spanID, ok := span.IDs(); ok {
				ctx = observability.WithTrace(ctx, traceID, spanID)
			}

			c.SetRequest(req.WithContext(ctx))
			handlerErr := next(c)

			_, statusCode := echo.ResolveResponseStatus(c.Response(), handlerErr)
			span.SetAttributes(observability.FromPairs(keyHTTPResponseStatus, statusCode))

			originalError := httpcontext.Error(c)
			errorDetails := httpcontext.ErrorDetails(c)
			transportCode, transportMsg := httpcontext.TransportError(c)
			if originalError != nil {
				span.SetAttributes(observability.FromPairs(
					keyError, originalError.Error(),
					keyErrorChain, observability.ErrorCauseChain(originalError),
				))
			}
			if len(errorDetails) > 0 {
				span.SetAttributes(observability.FromPairs(keyErrorDetails, errorDetails.String()))
			}
			if transportCode != "" {
				span.SetAttributes(observability.FromPairs(
					keyTransportCode, transportCode,
					keyTransportMessage, transportMsg,
				))
			}

			span.Finish(originalError)
			return handlerErr
		}
	}
}

type headerCarrier struct{ http.Header }

func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.Header))
	for key := range c.Header {
		keys = append(keys, key)
	}
	return keys
}
