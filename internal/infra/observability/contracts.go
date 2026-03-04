package observability

import "context"

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
