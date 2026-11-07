package contextmw

import (
	"context"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Config bounds one processing attempt. App configuration validates a positive
// HandlingTimeout and a nonnegative AcknowledgmentMargin before construction.
type Config struct {
	HandlingTimeout time.Duration
	// Reserve time to acknowledge a Share Group record before its lock expires.
	AcknowledgmentMargin time.Duration
}

// Middleware carries trace metadata and applies a processing deadline while
// preserving worker cancellation. Handlers must honor context cancellation.
type Middleware struct {
	tracer observability.Tracer
	config Config
}

func New(tracer observability.Tracer, config Config) *Middleware {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return &Middleware{tracer: tracer, config: config}
}

func (m *Middleware) Handler() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, record *kgo.Record) error {
			ctx = recordContext(ctx, m.tracer, record)
			deadline := time.Now().Add(m.config.HandlingTimeout)
			// Share Groups temporarily lock each fetched record for this consumer.
			// If the lock expires before Kafka receives our acknowledgment, another
			// consumer can get the record. Finish before the SDK's estimated lock
			// deadline, leaving margin for sending the acknowledgment.
			//
			// Traditional groups assign whole partitions and have no per-record
			// lock. AcquisitionDeadline returns zero for those records, so they
			// use the configured handling timeout (or an earlier parent deadline).
			// Their partition ownership can still change through a rebalance or
			// session timeout; this middleware does not detect those changes.
			if lock := record.AcquisitionDeadline(); !lock.IsZero() {
				if finishBy := lock.Add(-m.config.AcknowledgmentMargin); finishBy.Before(deadline) {
					deadline = finishBy
				}
			}
			ctx, cancel := context.WithDeadline(ctx, deadline)
			defer cancel()
			if err := ctx.Err(); err != nil {
				return err
			}
			err := next(ctx, record)
			// A handler must honor cancellation. A late nil return is not safe to accept.
			if err == nil {
				err = ctx.Err()
			}
			return err
		}
	}
}

// Kotel stores its receive span in record.Context. Transfer that trace onto the
// worker context so shutdown cancellation and handler deadlines still apply.
// Header extraction also supports callers using a reader without kotel hooks.
func recordContext(ctx context.Context, tracer observability.Tracer, record *kgo.Record) context.Context {
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
