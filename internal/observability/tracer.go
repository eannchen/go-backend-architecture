package observability

import (
	"context"
)

// TextMapCarrier carries distributed-tracing metadata without tying the
// observability contract to a transport framework.
type TextMapCarrier interface {
	Get(key string) string
	Set(key, value string)
	Keys() []string
}

// Span is an observability-agnostic span contract for app layers.
type Span interface {
	SetAttributes(fields ...Fields)
	Finish(err error, description ...string)
}

// TraceContext identifies the active span without exposing vendor types.
type TraceContext struct {
	TraceID string
	SpanID  string
}

// Tracer is injected into app layers to avoid direct OTel dependency.
type Tracer interface {
	Start(ctx context.Context, scope, spanName string, fields ...Fields) (context.Context, Span)
	StartServer(ctx context.Context, scope, spanName string, fields ...Fields) (context.Context, Span)
	StartClient(ctx context.Context, scope, spanName string, fields ...Fields) (context.Context, Span)
	Extract(ctx context.Context, carrier TextMapCarrier) context.Context
	Inject(ctx context.Context, carrier TextMapCarrier)
	TraceContext(ctx context.Context) (TraceContext, bool)
}

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
