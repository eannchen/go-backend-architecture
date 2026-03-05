package logger

import "context"

// Field is a logger-agnostic key/value pair.
type Field struct {
	Key   string
	Value any
}

// FieldOf creates one structured field.
func FieldOf(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Fields creates fields from key/value pairs.
//
// Example:
//
//	Fields("address", ":8080", "component", "http_server")
//
// If pairs length is odd, the last dangling key is ignored.
func Fields(pairs ...any) []Field {
	out := make([]Field, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok || key == "" {
			continue
		}
		out = append(out, FieldOf(key, pairs[i+1]))
	}
	return out
}

type Severity uint8

const (
	SeverityDebug Severity = iota
	SeverityInfo
	SeverityWarn
	SeverityError
)

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
