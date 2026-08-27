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
	if meter == nil {
		meter = observability.NoopMeter{}
	}
	return &RequestMetrics{
		requests: meter.Counter("http_server_requests_total", observability.MetricOption{Description: "Completed HTTP requests.", Unit: "{request}"}),
		errors:   meter.Counter("http_server_errors_total", observability.MetricOption{Description: "Completed HTTP requests with 4xx or 5xx status.", Unit: "{error}"}),
		duration: meter.Histogram("http_server_request_duration_seconds", observability.MetricOption{Description: "HTTP request latency.", Unit: "s"}),
	}
}

// Record records one completed HTTP request.
func (m *RequestMetrics) Record(ctx context.Context, outcome requestOutcome) {
	fields := metricFields(outcome.request, outcome.status)
	m.requests.Add(ctx, 1, fields)
	m.duration.Record(ctx, outcome.duration.Seconds(), fields)
	if outcome.status >= 400 {
		m.errors.Add(ctx, 1, fields)
	}
}

// metricFields intentionally excludes the concrete URL path to prevent one
// metric series per resource identifier or unknown URL.
func metricFields(request requestInfo, status int) observability.Fields {
	return observability.MergeFields(
		request.fields(),
		observability.FromPairs(keyHTTPResponseStatus, status),
	)
}
