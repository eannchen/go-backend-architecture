package observabilitymw

import (
	"context"
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
	ctx = t.tracer.Extract(ctx, headerCarrier{Header: request.propagationHeaders})
	return t.tracer.StartServer(
		ctx,
		instrumentationScope,
		request.spanName(),
		request.spanStartFields(),
	)
}

// Finish records the normalized outcome and ends the span.
func (*Tracing) Finish(span observability.Span, outcome requestOutcome) {
	fields := observability.FromPairs(keyHTTPResponseStatusCode, outcome.responseStatusCode)
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
	span.Finish(outcome.applicationError.originalError)
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
