package observability

import (
	"context"
)

// TextMapCarrier carries distributed-tracing metadata without tying the
// observability contract to a transport framework.
// Adapters implement it over HTTP headers, gRPC metadata, or Kafka headers.
// The current tracer uses W3C traceparent and optional tracestate fields.
type TextMapCarrier interface {
	// Get returns a field's value, or an empty string if it is absent.
	Get(key string) string
	// Set writes a field, replacing its previous value.
	Set(key, value string)
	// Keys lists the available field names.
	Keys() []string
}

// Span measures one operation within a trace, such as a request or database call.
// The code that starts it must call Finish once when the operation completes.
type Span interface {
	// SetAttributes adds diagnostic fields, updating existing fields with the same
	// keys. Pass one Fields map; the current implementation uses only the first.
	// Exclude credentials and sensitive payloads from attributes and errors.
	SetAttributes(fields ...Fields)
	// Finish records the outcome and ends the span's duration. Pass nil on success.
	// The current implementation records errors as events. Errors reporting
	// IsClientError() == true retain a successful span status; other errors mark
	// the span as failed. The first description overrides the failure status text
	// if nonempty; otherwise err.Error() is used. Further descriptions are ignored.
	// The original error is still recorded, so it must also be safe.
	// Finishing a span does not cancel its context or stop the operation's code.
	Finish(err error, description ...string)
}

// TraceContext identifies the active span without exposing vendor types.
// It contains IDs for correlation, for example when adding trace IDs to logs.
// Use Inject and Extract to propagate the full tracing metadata across services.
type TraceContext struct {
	// TraceID is shared by spans belonging to the same distributed operation.
	TraceID string
	// SpanID identifies one span within that trace.
	SpanID string
}

// Tracer is injected into app layers to avoid direct OTel dependency.
// A trace connects related operations; each span measures one of those operations.
//
// All Start methods return a context containing the new span. Pass that context
// downstream so nested spans become its children. They preserve the incoming
// context's values, cancellation, and deadline. With a valid parent, the new span
// keeps its trace ID and gets a new span ID; without one, a new trace begins.
// Recording and export depend on the configured telemetry and sampling policy.
type Tracer interface {
	// Start begins an internal operation, such as usecase or message processing.
	// scope identifies the instrumented component, e.g. "delivery.kafka".
	// spanName names the operation, e.g. "notification.consume". Choose stable
	// names and put per-record details in attributes. Pass one optional Fields map;
	// the current implementation uses only the first. These arguments have the
	// same meaning for StartServer and StartClient. Finish the returned span.
	Start(ctx context.Context, scope, spanName string, fields ...Fields) (context.Context, Span)
	// StartServer begins handling an incoming server request, such as HTTP or gRPC.
	// Extract incoming trace metadata first to connect it to the caller's trace.
	// Finish the returned span when request handling completes.
	StartServer(ctx context.Context, scope, spanName string, fields ...Fields) (context.Context, Span)
	// StartClient begins an outgoing call, such as HTTP, gRPC, or database I/O.
	// For transports supporting trace propagation, Inject the returned context
	// into the outgoing headers. Finish the span when the call completes.
	StartClient(ctx context.Context, scope, spanName string, fields ...Fields) (context.Context, Span)
	// Extract reads tracing metadata from the carrier into a derived context,
	// preserving ctx's cancellation, deadline, and other values. It creates no
	// span. A subsequent Start method uses the extracted identity as its parent.
	// Missing or invalid metadata leaves ctx unchanged in the current tracer.
	// A nil carrier is allowed and also leaves ctx unchanged.
	Extract(ctx context.Context, carrier TextMapCarrier) context.Context
	// Inject writes ctx's tracing metadata into the carrier for sending downstream.
	// It creates no span and does not send anything itself. With no valid trace
	// identity or a nil carrier, the current tracer writes nothing.
	Inject(ctx context.Context, carrier TextMapCarrier)
	// TraceContext reads the trace and span IDs stored in ctx. The bool reports
	// whether they are valid, including identities extracted from headers or from
	// an ended span. Valid IDs do not guarantee that the span is recorded/exported.
	// It creates no span. Use the returned IDs only when the bool is true.
	TraceContext(ctx context.Context) (TraceContext, bool)
}

// NoopTracer disables tracing. Start and Extract return the original context,
// Inject writes nothing, and TraceContext returns false even if ctx carries IDs.
// Its returned spans accept SetAttributes and Finish without doing any work.
type NoopTracer struct{}

type noopSpan struct{}

func (NoopTracer) Start(ctx context.Context, _ string, _ string, _ ...Fields) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (NoopTracer) StartServer(ctx context.Context, _ string, _ string, _ ...Fields) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (NoopTracer) StartClient(ctx context.Context, _ string, _ string, _ ...Fields) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (NoopTracer) Extract(ctx context.Context, _ TextMapCarrier) context.Context {
	return ctx
}

func (NoopTracer) Inject(context.Context, TextMapCarrier) {}

func (NoopTracer) TraceContext(context.Context) (TraceContext, bool) {
	return TraceContext{}, false
}

func (noopSpan) SetAttributes(_ ...Fields) {}

func (noopSpan) Finish(_ error, _ ...string) {}
