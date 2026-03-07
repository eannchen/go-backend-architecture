package observability

import (
	"context"
	"net/http"
)

// Span is an observability-agnostic span contract for app layers.
type Span interface {
	SetAttributes(fields ...Field)
	Finish(err error, description ...string)
	IDs() (traceID, spanID string, ok bool)
}

// Tracer is injected into app layers to avoid direct OTel dependency.
type Tracer interface {
	Start(ctx context.Context, scope, spanName string, fieldSets ...[]Field) (context.Context, Span)
	StartServer(ctx context.Context, scope, spanName string, fieldSets ...[]Field) (context.Context, Span)
	ExtractHTTP(ctx context.Context, headers http.Header) context.Context
}

type NoopTracer struct{}

type noopSpan struct{}

func (NoopTracer) Start(ctx context.Context, _ string, _ string, _ ...[]Field) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (NoopTracer) StartServer(ctx context.Context, _ string, _ string, _ ...[]Field) (context.Context, Span) {
	return ctx, noopSpan{}
}

func (NoopTracer) ExtractHTTP(ctx context.Context, _ http.Header) context.Context {
	return ctx
}

func (noopSpan) SetAttributes(_ ...Field) {}

func (noopSpan) Finish(_ error, _ ...string) {}

func (noopSpan) IDs() (traceID, spanID string, ok bool) {
	return "", "", false
}
