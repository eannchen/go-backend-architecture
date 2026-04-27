package logger

import "context"

// LogSinkFunc is an optional secondary sink for structured logs.
// The primary sink can stay terminal/stdout; this sink can export to any backend without coupling the logger API.
type LogSinkFunc func(ctx context.Context, severity Severity, message string, fields ...Fields)

// ContextFieldsProviderFunc extracts context-bound fields (e.g. request/trace IDs) for every log and sink export.
type ContextFieldsProviderFunc func(ctx context.Context) Fields

// Logger defines the project logging contract.
// Context is included in every call so tracing metadata can be attached later without changing call sites.
type Logger interface {
	Debug(ctx context.Context, message string, fields ...Fields)
	Info(ctx context.Context, message string, fields ...Fields)
	Warn(ctx context.Context, message string, fields ...Fields)
	Error(ctx context.Context, message string, err error, fields ...Fields)
	// ErrorNoStack logs at error level without stack trace for expected failures.
	ErrorNoStack(ctx context.Context, message string, err error, fields ...Fields)
	// SetLogSink sets a secondary log sink. Must be called during startup before concurrent logging begins.
	SetLogSink(sink LogSinkFunc)
	// SetContextFieldsProvider sets the provider for context-bound fields. Must be called during startup before concurrent logging begins.
	SetContextFieldsProvider(provider ContextFieldsProviderFunc)
	Sync() error
}

// NoopLogger is a no-op implementation of Logger.
type NoopLogger struct{}

func (n NoopLogger) Debug(ctx context.Context, message string, fields ...Fields)            {}
func (n NoopLogger) Info(ctx context.Context, message string, fields ...Fields)             {}
func (n NoopLogger) Warn(ctx context.Context, message string, fields ...Fields)             {}
func (n NoopLogger) Error(ctx context.Context, message string, err error, fields ...Fields) {}
func (n NoopLogger) ErrorNoStack(ctx context.Context, message string, err error, fields ...Fields) {
}
func (n NoopLogger) SetLogSink(sink LogSinkFunc)                                 {}
func (n NoopLogger) SetContextFieldsProvider(provider ContextFieldsProviderFunc) {}
func (n NoopLogger) Sync() error                                                 { return nil }
