package logger

import "context"

// Field is a logger-agnostic key/value pair.
type Field struct {
	Key   string
	Value any
}

// LogSinkFunc is an optional secondary sink for structured logs.
//
// The primary sink can stay terminal/stdout, while this sink can export logs
// to any backend (OTel, Kafka, file, etc.) without coupling logger API to it.
type LogSinkFunc func(ctx context.Context, severityText, message string, fields ...Field)

// Logger defines the project logging contract.
//
// Context is included in every call so tracing metadata can be attached later
// without changing call sites.
type Logger interface {
	Debug(ctx context.Context, message string, fields ...Field)
	Info(ctx context.Context, message string, fields ...Field)
	Warn(ctx context.Context, message string, fields ...Field)
	Error(ctx context.Context, message string, err error, fields ...Field)
	SetLogSink(sink LogSinkFunc)
	Sync() error
}
