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

	// Duration follows OTel RPC conventions. Counts and active streams are custom.
	return &RequestMetrics{
		requests:      meter.Counter("app.grpc.server.requests", appobservability.MetricOption{Description: "Completed gRPC requests.", Unit: "{request}"}),
		errors:        meter.Counter("app.grpc.server.errors", appobservability.MetricOption{Description: "Completed gRPC requests with a non-OK status.", Unit: "{error}"}),
		duration:      meter.Histogram("rpc.server.call.duration", appobservability.MetricOption{Description: "gRPC request duration.", Unit: "s"}),
		activeStreams: meter.UpDownCounter("app.grpc.server.active_streams", appobservability.MetricOption{Description: "Currently active gRPC streams.", Unit: "{stream}"}),
	}
}

// Record records one completed RPC.
func (m *RequestMetrics) Record(ctx context.Context, outcome rpcOutcome) {
	fields := metricFields(outcome)
	m.requests.Add(ctx, 1, fields)
	m.duration.Record(ctx, outcome.duration.Seconds(), fields)
	if outcome.responseStatusCode != codes.OK {
		m.errors.Add(ctx, 1, fields)
	}
}

// StreamStarted increments the active-stream count.
func (m *RequestMetrics) StreamStarted(ctx context.Context, rpc rpcInfo) {
	m.activeStreams.Add(ctx, 1, rpc.spanStartFields())
}

// StreamFinished decrements the active-stream count and records its outcome.
func (m *RequestMetrics) StreamFinished(ctx context.Context, outcome rpcOutcome) {
	m.activeStreams.Add(ctx, -1, outcome.rpc.spanStartFields())
	m.Record(ctx, outcome)
}

// metricFields derives bounded attributes once for all completed-call metrics.
func metricFields(outcome rpcOutcome) appobservability.Fields {
	fields := outcome.rpc.spanStartFields()
	fields[keyRPCStatusCode] = outcome.responseStatusName()
	if errorType := outcome.errorType(); errorType != "" {
		fields[keyErrorType] = errorType
	}
	return fields
}
