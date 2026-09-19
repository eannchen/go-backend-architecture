package observability

import (
	"context"

	"google.golang.org/grpc/codes"

	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// RequestMetrics records bounded client RPC outcome and stream-lifecycle metrics.
type RequestMetrics struct {
	requests      appobservability.Counter
	errors        appobservability.Counter
	duration      appobservability.Histogram
	activeStreams appobservability.UpDownCounter
}

// NewRequestMetrics creates the client instruments with a safe no-op default.
func NewRequestMetrics(meter appobservability.Meter) *RequestMetrics {
	if meter == nil {
		meter = appobservability.NoopMeter{}
	}
	return &RequestMetrics{
		requests:      meter.Counter("grpc_client_requests_total", appobservability.MetricOption{Description: "Completed outbound gRPC requests.", Unit: "{request}"}),
		errors:        meter.Counter("grpc_client_errors_total", appobservability.MetricOption{Description: "Outbound gRPC requests with a non-OK status.", Unit: "{error}"}),
		duration:      meter.Histogram("grpc_client_request_duration_seconds", appobservability.MetricOption{Description: "Outbound gRPC request duration.", Unit: "s"}),
		activeStreams: meter.UpDownCounter("grpc_client_active_streams", appobservability.MetricOption{Description: "Currently active outbound gRPC streams.", Unit: "{stream}"}),
	}
}

// Record observes one completed RPC.
func (m *RequestMetrics) Record(ctx context.Context, outcome rpcOutcome) {
	fields := metricFields(outcome.rpc, outcome.responseStatusName())
	m.requests.Add(ctx, 1, fields)
	m.duration.Record(ctx, outcome.duration.Seconds(), fields)
	if outcome.responseStatusCode != codes.OK {
		m.errors.Add(ctx, 1, fields)
	}
}

// StreamStarted increments the active stream count after a call begins.
func (m *RequestMetrics) StreamStarted(ctx context.Context, rpc rpcInfo) {
	m.activeStreams.Add(ctx, 1, rpc.spanStartFields())
}

// StreamFinished decrements the active count and records the terminal outcome.
func (m *RequestMetrics) StreamFinished(ctx context.Context, outcome rpcOutcome) {
	m.activeStreams.Add(ctx, -1, outcome.rpc.spanStartFields())
	m.Record(ctx, outcome)
}

func metricFields(rpc rpcInfo, status string) appobservability.Fields {
	return appobservability.MergeFields(rpc.spanStartFields(), appobservability.FromPairs(keyRPCResponseStatusCode, status))
}
