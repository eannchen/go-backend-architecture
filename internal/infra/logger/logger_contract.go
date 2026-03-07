package logger

import "context"

// LogSinkFunc is an optional secondary sink for structured logs.
//
// The primary sink can stay terminal/stdout, while this sink can export logs
// to any backend (OTel, Kafka, file, etc.) without coupling logger API to it.
type LogSinkFunc func(ctx context.Context, severity Severity, message string, fields ...Fields)

// ContextFieldsProviderFunc extracts context-bound fields (e.g. request/trace IDs)
// that should be attached to every log and sink export.
type ContextFieldsProviderFunc func(ctx context.Context) Fields

// Logger defines the project logging contract.
//
// Context is included in every call so tracing metadata can be attached later
// without changing call sites.
type Logger interface {
	// fields is optional. When multiple field sets are passed, only the first one is used.
	Debug(ctx context.Context, message string, fields ...Fields)
	Info(ctx context.Context, message string, fields ...Fields)
	Warn(ctx context.Context, message string, fields ...Fields)
	Error(ctx context.Context, message string, err error, fields ...Fields)
	SetLogSink(sink LogSinkFunc)
	SetContextFieldsProvider(provider ContextFieldsProviderFunc)
	Sync() error
}
