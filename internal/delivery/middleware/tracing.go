package middleware

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"go-backend-architecture/internal/infra/observability"
)

func Tracing(tracer observability.Tracer) echo.MiddlewareFunc {
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

			statusCode := http.StatusOK
			if sw, ok := c.Response().(interface{ Status() int }); ok {
				statusCode = sw.Status()
			}
			span.SetAttributes(observability.FromPairs("http.response.status_code", statusCode))
			span.Finish(err)

			return err
		}
	}
}
