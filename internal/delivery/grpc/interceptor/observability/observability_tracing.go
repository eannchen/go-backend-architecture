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
		// Extract converts wire-level traceparent/tracestate metadata into an OTel
		// remote parent stored in ctx; it does not create the server span itself.
		ctx = t.tracer.Extract(ctx, metadataCarrier{MD: md})
	}
	// StartServer always creates this service's own span. A valid extracted
	// parent joins it to the upstream trace; no parent starts a new root trace.
	return t.tracer.StartServer(ctx, instrumentationScope, rpc.method, rpc.spanStartFields())
}

// Finish records the normalized outcome and ends the span.
func (*Tracing) Finish(span appobservability.Span, outcome rpcOutcome) {
	fields := appobservability.FromPairs(keyRPCResponseStatusCode, outcome.responseStatusName())
	if errorType := outcome.errorType(); errorType != "" {
		fields[keyErrorType] = errorType
	}
	if outcome.applicationError.causeChain != "" {
		fields[keyApplicationErrorCauseChain] = outcome.applicationError.causeChain
	}
	if outcome.applicationError.diagnosticDetails != "" {
		fields[keyApplicationErrorDetails] = outcome.applicationError.diagnosticDetails
	}
	if outcome.applicationError.applicationErrorCode != "" {
		fields[keyApplicationErrorCode] = outcome.applicationError.applicationErrorCode
	}
	if outcome.applicationError.applicationErrorMessage != "" {
		fields[keyApplicationErrorMessage] = outcome.applicationError.applicationErrorMessage
	}
	span.SetAttributes(fields)
	span.Finish(outcome.handlerError)
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
