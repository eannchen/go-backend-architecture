package observability

import (
	"context"

	"google.golang.org/grpc/codes"

	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// RequestMetrics records bounded-cardinality gRPC server metrics.
type RequestMetrics struct {
	requests      appobservability.Counter
	errors        appobservability.Counter
	duration      appobservability.Histogram
	activeStreams appobservability.UpDownCounter
}

// NewRequestMetrics creates gRPC server request metrics.
func NewRequestMetrics(meter appobservability.Meter) *RequestMetrics {
	if meter == nil {
		meter = appobservability.NoopMeter{}
	}
	return &RequestMetrics{
		requests:      meter.Counter("grpc_server_requests_total", appobservability.MetricOption{Description: "Completed gRPC requests.", Unit: "{request}"}),
		errors:        meter.Counter("grpc_server_errors_total", appobservability.MetricOption{Description: "Completed gRPC requests with a non-OK status.", Unit: "{error}"}),
		duration:      meter.Histogram("grpc_server_request_duration_seconds", appobservability.MetricOption{Description: "gRPC request latency.", Unit: "s"}),
		activeStreams: meter.UpDownCounter("grpc_server_active_streams", appobservability.MetricOption{Description: "Currently active gRPC streams.", Unit: "{stream}"}),
	}
}

// Record records one completed RPC.
func (m *RequestMetrics) Record(ctx context.Context, outcome rpcOutcome) {
	fields := metricFields(outcome.rpc, int(outcome.status))
	m.requests.Add(ctx, 1, fields)
	m.duration.Record(ctx, outcome.duration.Seconds(), fields)
	if outcome.status != codes.OK {
		m.errors.Add(ctx, 1, fields)
	}
}

// StreamStarted increments the active-stream count.
func (m *RequestMetrics) StreamStarted(ctx context.Context, rpc rpcInfo) {
	m.activeStreams.Add(ctx, 1, rpc.fields())
}

// StreamFinished decrements the active-stream count and records its outcome.
func (m *RequestMetrics) StreamFinished(ctx context.Context, outcome rpcOutcome) {
	m.activeStreams.Add(ctx, -1, outcome.rpc.fields())
	m.Record(ctx, outcome)
}

func metricFields(rpc rpcInfo, status int) appobservability.Fields {
	return appobservability.MergeFields(
		rpc.fields(),
		appobservability.FromPairs(keyGRPCStatusCode, status),
	)
}
