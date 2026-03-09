package middleware

import (
	"fmt"

	"github.com/labstack/echo/v5"

	"go-backend-architecture/internal/observability"
)

func Tracing(tracer observability.Tracer) echo.MiddlewareFunc {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := tracer.ExtractHTTP(req.Context(), req.Header)

			route := c.Path()
			if route == "" {
				route = req.URL.Path
			}

			spanName := fmt.Sprintf("%s %s", req.Method, route)
			ctx, span := tracer.StartServer(
				ctx,
				"http",
				spanName,
				observability.FromPairs(
					"http.request.method", req.Method,
					"http.route", route,
					"url.path", req.URL.Path,
				),
			)
			if traceID, spanID, ok := span.IDs(); ok {
				ctx = observability.WithTrace(ctx, traceID, spanID)
			}

			c.SetRequest(req.WithContext(ctx))

			err := next(c)

			_, statusCode := echo.ResolveResponseStatus(c.Response(), err)
			span.SetAttributes(observability.FromPairs("http.response.status_code", statusCode))
			span.Finish(err)

			return err
		}
	}
}
