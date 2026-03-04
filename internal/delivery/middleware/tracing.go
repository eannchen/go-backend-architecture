package middleware

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"vocynex-api/internal/infra/observability"
)

func Tracing(serviceName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := propagation.TraceContext{}.Extract(req.Context(), propagation.HeaderCarrier(req.Header))

			route := c.Path()
			if route == "" {
				route = req.URL.Path
			}

			spanName := fmt.Sprintf("%s %s", req.Method, route)
			ctx, span := observability.StartSpanWithOptions(
				ctx,
				fmt.Sprintf("%s/http", serviceName),
				spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.request.method", req.Method),
					attribute.String("http.route", route),
					attribute.String("url.path", req.URL.Path),
				),
			)
			defer span.End()

			spanCtx := span.SpanContext()
			if spanCtx.IsValid() {
				ctx = observability.WithTrace(ctx, spanCtx.TraceID().String(), spanCtx.SpanID().String())
			}

			c.SetRequest(req.WithContext(ctx))

			err := next(c)

			statusCode := http.StatusOK
			if sw, ok := c.Response().(interface{ Status() int }); ok {
				statusCode = sw.Status()
			}
			span.SetAttributes(attribute.Int("http.response.status_code", statusCode))
			if err != nil {
				span.Fail(err, err.Error())
			} else {
				span.OK()
			}

			return err
		}
	}
}
