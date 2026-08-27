package observability

import (
	"context"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// Tracing owns distributed-context extraction and server-span lifecycle.
type Tracing struct {
	tracer appobservability.Tracer
}

// NewTracing creates gRPC server tracing.
func NewTracing(tracer appobservability.Tracer) *Tracing {
	if tracer == nil {
		tracer = appobservability.NoopTracer{}
	}
	return &Tracing{tracer: tracer}
}

// Start extracts parent trace context and starts a gRPC server span.
func (t *Tracing) Start(ctx context.Context, rpc rpcInfo) (context.Context, appobservability.Span) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ctx = t.tracer.Extract(ctx, metadataCarrier{MD: md})
	}
	ctx, span := t.tracer.StartServer(ctx, instrumentationScope, rpc.fullMethod, rpc.fields())
	if traceID, spanID, ok := span.IDs(); ok {
		ctx = appobservability.WithTrace(ctx, traceID, spanID)
	}
	return ctx, span
}

// Finish records the normalized outcome and ends the span.
func (*Tracing) Finish(span appobservability.Span, outcome rpcOutcome) {
	fields := appobservability.FromPairs(keyGRPCStatusCode, int(outcome.status))
	if outcome.errorInfo.original != nil {
		fields[keyError] = outcome.errorInfo.original.Error()
		fields[keyErrorChain] = outcome.errorInfo.chain
	}
	if outcome.errorInfo.details != "" {
		fields[keyErrorDetails] = outcome.errorInfo.details
	}
	if outcome.errorInfo.code != "" {
		fields[keyErrorCode] = outcome.errorInfo.code
	}
	if outcome.errorInfo.message != "" {
		fields[keyErrorMessage] = outcome.errorInfo.message
	}
	span.SetAttributes(fields)
	span.Finish(outcome.handlerErr)
}

// metadataCarrier adapts multi-value gRPC metadata to the tracing carrier's
// scalar Get and Set methods and supplies key enumeration.
type metadataCarrier struct{ metadata.MD }

var _ appobservability.TextMapCarrier = metadataCarrier{}

func (c metadataCarrier) Get(key string) string {
	values := c.MD.Get(key)
	if len(values) == 0 {
		return ""
	}
	// Trace propagation fields such as traceparent are single-value fields;
	// select the first value when adapting gRPC's multi-value metadata.
	return values[0]
}

func (c metadataCarrier) Set(key, value string) { c.MD.Set(key, value) }

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c.MD))
	for key := range c.MD {
		keys = append(keys, key)
	}
	return keys
}

type contextServerStream struct {
	googlegrpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }
