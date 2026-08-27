package observabilitymw

import (
	"context"
	"fmt"
	"net/http"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Tracing owns distributed-context extraction and server-span lifecycle.
type Tracing struct {
	tracer observability.Tracer
}

// NewTracing creates HTTP server tracing.
func NewTracing(tracer observability.Tracer) *Tracing {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return &Tracing{tracer: tracer}
}

// Start extracts parent trace context and starts an HTTP server span.
func (t *Tracing) Start(ctx context.Context, request requestInfo) (context.Context, observability.Span) {
	ctx = t.tracer.Extract(ctx, headerCarrier{Header: request.header})
	ctx, span := t.tracer.StartServer(
		ctx,
		instrumentationScope,
		fmt.Sprintf("%s %s", request.method, request.route),
		observability.MergeFields(
			request.fields(),
			observability.FromPairs(keyURLPath, request.path),
		),
	)
	if traceID, spanID, ok := span.IDs(); ok {
		ctx = observability.WithTrace(ctx, traceID, spanID)
	}
	return ctx, span
}

// Finish records the normalized outcome and ends the span.
func (*Tracing) Finish(span observability.Span, outcome requestOutcome) {
	fields := observability.FromPairs(keyHTTPResponseStatus, outcome.status)
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
	span.Finish(outcome.errorInfo.original)
}

// headerCarrier adapts HTTP headers to the transport-neutral tracing carrier.
// Embedded http.Header already supplies compatible scalar Get and Set methods;
// only key enumeration needs an adapter.
type headerCarrier struct{ http.Header }

var _ observability.TextMapCarrier = headerCarrier{}

func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.Header))
	for key := range c.Header {
		keys = append(keys, key)
	}
	return keys
}
