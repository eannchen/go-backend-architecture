package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

// MessageReader is what this Kafka worker needs from its reader.
// It receives franz-go records, but maps their JSON into a plain Request before
// invoking the usecase. The usecase does not need to know Kafka or franz-go.
type MessageReader interface {
	FetchRecord(context.Context) (*kgo.Record, error)
	CommitRecords(context.Context, ...*kgo.Record) error
	AllowRebalance()
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

// Run repeats: get one message, handle it, save progress if successful.
// It finishes each message before asking for the next, even when assigned several
// partitions. This keeps the example easy to follow and avoids saving progress
// past unfinished work. A larger fetch setting does not make handlers run in parallel.
func (w *NotificationWorker) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		// Step 1: wait for a message without marking it as handled. The reader
		// briefly holds up partition moves until handle finishes this attempt.
		message, err := w.reader.FetchRecord(ctx)
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
			// Stop the worker and report where handling failed. Do not save progress
			// for this message. Restarting the same group can read it again, so a bad
			// message will keep failing until it is fixed or given a recovery policy.
			return fmt.Errorf("notification worker topic=%s partition=%d offset=%d: %w", message.Topic, message.Partition, message.Offset, err)
		}
	}
}

func (w *NotificationWorker) handle(ctx context.Context, message *kgo.Record) (err error) {
	// Step 2: arrange to let partitions move again when this attempt finishes.
	// "defer" runs the call when this function returns, including on errors.
	// AllowRebalance does not save progress; only CommitRecords can advance
	// the group's saved position.
	defer w.reader.AllowRebalance()
	// Step 3: continue the receive span that kotel placed on the record, while
	// keeping cancellation from our worker context. This span measures the
	// processing attempt, separately from Kafka's network and buffering work.
	ctx = notificationRecordContext(ctx, w.tracer, message)
	ctx, span := w.tracer.Start(ctx, "delivery.kafka", "notification.consume")
	defer func() { span.Finish(err) }()
	// Step 4: decode and validate the JSON, then invoke the notification usecase.
	// Any error returns before CommitRecords, so Kafka progress is not advanced.
	request, err := decodeNotification(message.Value)
	if err != nil {
		return err
	}
	if err := w.usecase.Handle(ctx, request); err != nil {
		return fmt.Errorf("handle notification: %w", err)
	}
	// Step 5: handling succeeded, so ask Kafka to save the group's next position.
	// Example: after handling offset 7, commit saves 8 as the next message to read.
	// If we crash after handling but before this commit succeeds, Kafka can give
	// us offset 7 again. This possibility is called "at-least-once processing".
	// Before adding real sending, we must decide how to avoid sending duplicates.
	// Cancellation can interrupt the commit too; do not assume progress was saved.
	if err := w.reader.CommitRecords(ctx, message); err != nil {
		return fmt.Errorf("commit notification: %w", err)
	}
	return nil
}
