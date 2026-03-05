package observability

import (
	"context"
	"net/http"
)

// ShutdownFunc closes telemetry pipelines gracefully.
type ShutdownFunc func(ctx context.Context) error

// Field is an observability-agnostic key/value attribute.
type Field struct {
	Key   string
	Value any
}

// Attr creates one key/value attribute.
func Attr(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Attrs creates attributes from key/value pairs.
//
// Example:
//   Attrs("http.method", "GET", "http.route", "/healthz")
//
// If pairs length is odd, the last dangling key is ignored.
func Attrs(pairs ...any) []Field {
	out := make([]Field, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok || key == "" {
			continue
		}
		out = append(out, Attr(key, pairs[i+1]))
	}
	return out
}

// LogEmitter is the contract for a secondary log sink.
type LogEmitter interface {
	Emit(ctx context.Context, severityText, message string, attrs ...Field)
}

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
