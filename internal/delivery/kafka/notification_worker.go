package kafka

import (
	"context"
	"errors"
	"fmt"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

// MessageReader is adapter-specific: Kafka types stay in the transport boundary.
// The app supplies the infrastructure reader; the usecase sees only Request.
type MessageReader interface {
	FetchMessage(context.Context) (kafkago.Message, error)
	CommitMessages(context.Context, ...kafkago.Message) error
}

type NotificationWorker struct {
	reader  MessageReader
	usecase notification.Usecase
	tracer  observability.Tracer
}

func NewNotificationWorker(reader MessageReader, usecase notification.Usecase, tracer observability.Tracer) *NotificationWorker {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return &NotificationWorker{reader: reader, usecase: usecase, tracer: tracer}
}

// Run handles one record at a time, including across partitions. This is a
// simple starting point, not a throughput-oriented worker pool. In particular,
// it cannot commit a later record while an earlier handler is still running.
func (w *NotificationWorker) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		// FetchMessage reads without committing. ReadMessage would commit before
		// our handler finishes and could lose work when handling fails.
		message, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			return fmt.Errorf("notification worker fetch: %w", err)
		}
		if err := w.handle(ctx, message); err != nil {
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			// Fail visibly and leave this record uncommitted. Restarting the same group
			// can deliver it again; invalid records need a deliberate recovery policy.
			return fmt.Errorf("notification worker topic=%s partition=%d offset=%d: %w", message.Topic, message.Partition, message.Offset, err)
		}
	}
}

func (w *NotificationWorker) handle(ctx context.Context, message kafkago.Message) (err error) {
	headers := make(messageHeaders, len(message.Headers))
	for _, header := range message.Headers {
		headers.Set(header.Key, string(header.Value))
	}
	ctx = w.tracer.Extract(ctx, headers)
	ctx, span := w.tracer.Start(ctx, "delivery.kafka", "notification.consume")
	defer func() { span.Finish(err) }()
	request, err := decodeNotification(message.Value)
	if err != nil {
		return err
	}
	if err := w.usecase.Handle(ctx, request); err != nil {
		return fmt.Errorf("handle notification: %w", err)
	}
	// A successful commit records this group's resume position. A crash after
	// handling but before committing may repeat the handling: this is at-least-once
	// processing, not exactly-once notification sending. The handler must tolerate
	// duplicates before real external side effects are added.
	// Cancellation also cancels the commit wait; interrupted work may be replayed.
	if err := w.reader.CommitMessages(ctx, message); err != nil {
		return fmt.Errorf("commit notification: %w", err)
	}
	return nil
}

// Headers carry optional producer trace context without exposing SDK types to
// the observability contract. Extract does not change partition/group behavior.
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
