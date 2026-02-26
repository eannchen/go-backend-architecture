package logger

import "context"

// Field is a logger-agnostic key/value pair.
type Field struct {
	Key   string
	Value any
}

// Logger defines the project logging contract.
//
// Context is included in every call so tracing metadata can be attached later
// without changing call sites.
type Logger interface {
	Debug(ctx context.Context, message string, fields ...Field)
	Info(ctx context.Context, message string, fields ...Field)
	Warn(ctx context.Context, message string, fields ...Field)
	Error(ctx context.Context, message string, err error, fields ...Field)
	Sync() error
}
