package observability

import (
	"context"

	"google.golang.org/grpc/metadata"

	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// Tracing owns local client span lifecycle and optional outbound trace propagation.
type Tracing struct {
	tracer                appobservability.Tracer
	propagateTraceContext bool
}

// TraceConfig controls whether the remote service receives this span context.
type TraceConfig struct {
	PropagateContext bool
}

// NewTracing creates client tracing with a safe no-op default.
func NewTracing(tracer appobservability.Tracer, config TraceConfig) *Tracing {
	if tracer == nil {
		tracer = appobservability.NoopTracer{}
	}
	return &Tracing{tracer: tracer, propagateTraceContext: config.PropagateContext}
}

// Start begins a client span and, when enabled, injects its propagation fields into metadata.
func (t *Tracing) Start(ctx context.Context, rpc rpcInfo) (context.Context, appobservability.Span) {
	// StartClient creates a new span for this outbound RPC. It becomes a child
	// of the active span in ctx, or a root span when the caller has no trace.
	// The returned ctx contains the span, not serialized trace metadata.
	ctx, span := t.tracer.StartClient(ctx, instrumentationScope, rpc.fullMethod, rpc.fields())
	if !t.propagateTraceContext {
		return ctx, span
	}

	// gRPC keeps outbound headers in the context as metadata.MD. Preserve any
	// metadata the caller already attached, such as authorization or tenant
	// information, instead of creating a trace-only metadata collection.
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		// MD is a mutable map. Work on a copy so adding trace propagation fields
		// cannot alter metadata owned by the caller's parent context.
		md = md.Copy()
	}
	// The OTel propagator writes traceparent and, when present, tracestate into
	// this transport-neutral carrier backed by the copied gRPC metadata.
	t.tracer.Inject(ctx, metadataCarrier{MD: md})
	// Context values are not changed in place. Attach the completed metadata to
	// a derived context, which gRPC reads when it sends the outbound RPC.
	return metadata.NewOutgoingContext(ctx, md), span
}

// Finish records the bounded transport outcome and completes the span.
func (*Tracing) Finish(span appobservability.Span, outcome rpcOutcome) {
	fields := appobservability.FromPairs(keyGRPCStatusCode, int(outcome.status))
	if outcome.err != nil {
		fields[keyError] = outcome.err.Error()
		fields[keyErrorChain] = appobservability.ErrorCauseChain(outcome.err)
		fields[keyErrorMessage] = outcome.message
	}
	span.SetAttributes(fields)
	span.Finish(outcome.err, outcome.message)
}

// metadataCarrier adapts multi-value outgoing gRPC metadata to the tracing
// carrier's scalar methods while retaining key enumeration for propagators.
type metadataCarrier struct{ metadata.MD }

var _ appobservability.TextMapCarrier = metadataCarrier{}

func (c metadataCarrier) Get(key string) string {
	values := c.MD.Get(key)
	if len(values) == 0 {
		return ""
	}
	// W3C trace propagation fields are single-value fields, so the first value
	// is the scalar representation expected by the observability contract.
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
