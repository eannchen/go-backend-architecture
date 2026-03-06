package logger

import "context"

// LogSinkFunc is an optional secondary sink for structured logs.
//
// The primary sink can stay terminal/stdout, while this sink can export logs
// to any backend (OTel, Kafka, file, etc.) without coupling logger API to it.
type LogSinkFunc func(ctx context.Context, severity Severity, message string, fields ...Field)

// ContextFieldsProviderFunc extracts context-bound fields (e.g. request/trace IDs)
// that should be attached to every log and sink export.
type ContextFieldsProviderFunc func(ctx context.Context) []Field

// Logger defines the project logging contract.
//
// Context is included in every call so tracing metadata can be attached later
// without changing call sites.
type Logger interface {
	Debug(ctx context.Context, message string, fields ...[]Field)
	Info(ctx context.Context, message string, fields ...[]Field)
	Warn(ctx context.Context, message string, fields ...[]Field)
	Error(ctx context.Context, message string, err error, fields ...[]Field)
	SetLogSink(sink LogSinkFunc)
	SetContextFieldsProvider(provider ContextFieldsProviderFunc)
	Sync() error
}
