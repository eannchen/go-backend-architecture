package observability

import (
	"context"
	"net/http"
)

// Span is an observability-agnostic span contract for app layers.
type Span interface {
	SetAttributes(attrs ...Field)
	Fail(err error, description string)
	OK()
	End()
	IDs() (traceID, spanID string, ok bool)
}

// Tracer is injected into app layers to avoid direct OTel dependency.
type Tracer interface {
	Start(ctx context.Context, scope, spanName string, attrs ...[]Field) (context.Context, Span)
	StartServer(ctx context.Context, scope, spanName string, attrs ...[]Field) (context.Context, Span)
	ExtractHTTP(ctx context.Context, headers http.Header) context.Context
}
