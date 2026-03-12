package observabilitymw

import (
	"fmt"

	"github.com/labstack/echo/v5"

	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// TraceMiddleware manages request span lifecycle and response/error trace attributes.
type TraceMiddleware struct {
	tracer observability.Tracer
	meta   httpresponse.Meta
}

// NewTraceMiddleware creates trace middleware with response metadata.
func NewTraceMiddleware(tracer observability.Tracer, meta httpresponse.Meta) *TraceMiddleware {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	if meta == nil {
		meta = httpresponse.NewContextMeta()
	}
	return &TraceMiddleware{
		tracer: tracer,
		meta:   meta,
	}
}

// Handler builds the Echo middleware function for request tracing.
func (m *TraceMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := m.tracer.ExtractHTTP(req.Context(), req.Header)

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

			originalError := m.meta.GetError(c)
			errorDetails := m.meta.GetErrorDetails(c)
			transportCode, transportMsg := m.meta.GetTransportError(c)
			if originalError != nil {
				span.SetAttributes(observability.FromPairs(
					keyError, originalError.Error(),
					keyErrorChain, errorCauseChain(originalError),
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
