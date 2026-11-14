package observabilitymw

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// RequestMetrics records bounded-cardinality HTTP server metrics.
type RequestMetrics struct {
	requests observability.Counter
	errors   observability.Counter
	duration observability.Histogram
}

// NewRequestMetrics creates HTTP server request metrics.
func NewRequestMetrics(meter observability.Meter) *RequestMetrics {

	// Duration follows OTel HTTP conventions; the additional counters are custom.
	return &RequestMetrics{
		requests: meter.Counter("app.http.server.requests", observability.MetricOption{Description: "Completed HTTP requests.", Unit: "{request}"}),
		errors:   meter.Counter("app.http.server.errors", observability.MetricOption{Description: "Completed HTTP requests with 4xx or 5xx status.", Unit: "{error}"}),
		duration: meter.Histogram("http.server.request.duration", observability.MetricOption{Description: "HTTP request duration.", Unit: "s"}),
	}
}

// Record records one completed HTTP request.
func (m *RequestMetrics) Record(ctx context.Context, outcome requestOutcome) {
	fields := metricFields(outcome)
	m.requests.Add(ctx, 1, fields)
	m.duration.Record(ctx, outcome.duration.Seconds(), fields)
	if outcome.responseStatusCode >= 400 {
		m.errors.Add(ctx, 1, fields)
	}
}

// metricFields intentionally excludes the concrete URL path to prevent one
// metric series per resource identifier or unknown URL.
func metricFields(outcome requestOutcome) observability.Fields {
	request := outcome.request
	fields := observability.FromPairs(
		keyHTTPRequestMethod, request.requestMethod,
		keyURLScheme, request.urlScheme,
		keyHTTPResponseStatusCode, outcome.responseStatusCode,
	)
	if request.routeTemplate != "" {
		fields[keyHTTPRoute] = request.routeTemplate
	}
	if errorType := outcome.errorType(); errorType != "" {
		fields[keyErrorType] = errorType
	}
	return fields
}
