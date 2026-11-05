package kafka

import (
	"context"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Kotel stores its receive span in record.Context. Transfer that trace onto the
// worker context so shutdown cancellation and handler deadlines still apply.
// Header extraction also supports callers using a reader without kotel hooks.
func notificationRecordContext(ctx context.Context, tracer observability.Tracer, record *kgo.Record) context.Context {
	headers := make(messageHeaders, len(record.Headers))
	for _, header := range record.Headers {
		headers.Set(header.Key, string(header.Value))
	}
	if record.Context != nil {
		if _, valid := tracer.TraceContext(record.Context); valid {
			tracer.Inject(record.Context, headers)
		}
	}
	return tracer.Extract(ctx, headers)
}

// messageHeaders lets the tracing code read the message's key/value headers
// without depending on franz-go types. Trace headers only help track an operation;
// they do not choose partitions or change the consumer group's saved progress.
type messageHeaders map[string]string

func (h messageHeaders) Get(key string) string { return h[key] }
func (h messageHeaders) Set(key, value string) { h[key] = value }
func (h messageHeaders) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
