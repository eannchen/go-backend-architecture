package observability

import (
	"context"
)

// LogEmitter is the contract for a secondary log sink.
type LogEmitter interface {
	Emit(ctx context.Context, severity Severity, message string, fields ...Field)
}

// NoopLogEmitter is a no-op implementation of the LogEmitter contract.
type NoopLogEmitter struct{}

func (n NoopLogEmitter) Emit(context.Context, Severity, string, ...Field) {}
