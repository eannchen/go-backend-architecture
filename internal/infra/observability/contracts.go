package observability

import (
	"context"
	"net/http"
)

// ShutdownFunc closes telemetry pipelines gracefully.
type ShutdownFunc func(ctx context.Context) error

// KV is a logger-agnostic key/value attribute for a log sink.
type KV struct {
	Key   string
	Value any
}

// LogEmitter is the contract for a secondary log sink.
type LogEmitter interface {
	Emit(ctx context.Context, severityText, message string, attrs ...KV)
}

// Span is an observability-agnostic span contract for app layers.
type Span interface {
	SetAttributes(attrs ...KV)
	Fail(err error, description string)
	OK()
	End()
	IDs() (traceID, spanID string, ok bool)
}

// Tracer is injected into app layers to avoid direct OTel dependency.
type Tracer interface {
	Start(ctx context.Context, scope, spanName string, attrs ...KV) (context.Context, Span)
	StartServer(ctx context.Context, scope, spanName string, attrs ...KV) (context.Context, Span)
	ExtractHTTP(ctx context.Context, headers http.Header) context.Context
}
