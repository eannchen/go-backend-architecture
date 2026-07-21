package observabilitymw

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// RequestMetricsMiddleware records bounded-cardinality HTTP request metrics.
type RequestMetricsMiddleware struct {
	requests observability.Counter
	errors   observability.Counter
	latency  observability.Histogram
}

// NewRequestMetricsMiddleware creates HTTP request metrics.
func NewRequestMetricsMiddleware(meter observability.Meter) *RequestMetricsMiddleware {
	if meter == nil {
		meter = observability.NoopMeter{}
	}
	return &RequestMetricsMiddleware{
		requests: meter.Counter("http_server_requests_total", observability.MetricOption{Description: "Completed HTTP requests.", Unit: "{request}"}),
		errors:   meter.Counter("http_server_errors_total", observability.MetricOption{Description: "Completed HTTP requests with 4xx or 5xx status.", Unit: "{error}"}),
		latency:  meter.Histogram("http_server_request_duration_seconds", observability.MetricOption{Description: "HTTP request latency.", Unit: "s"}),
	}
}

// Handler returns the Echo middleware that records request outcome and latency.
func (m *RequestMetricsMiddleware) Handler() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			started := time.Now()
			err := next(c)
			_, status := echo.ResolveResponseStatus(c.Response(), err)
			route := c.Path()
			if route == "" {
				route = "unmatched"
			}
			fields := observability.FromPairs("http.request.method", c.Request().Method, "http.route", route, "http.response.status_code", strconv.Itoa(status))
			ctx := c.Request().Context()
			m.requests.Add(ctx, 1, fields)
			m.latency.Record(ctx, time.Since(started).Seconds(), fields)
			if status >= 400 {
				m.errors.Add(ctx, 1, fields)
			}
			return err
		}
	}
}
