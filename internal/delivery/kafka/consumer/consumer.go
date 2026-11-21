package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
)

// MessageReader is what this Kafka consumer needs from its reader.
// The injected handler selects the workflow and decodes its message.
type MessageReader interface {
	FetchRecord(context.Context) (*kgo.Record, error)
	CommitRecords(context.Context, ...*kgo.Record) error
	AllowRebalance()
}

type Consumer struct {
	reader  MessageReader
	handler middleware.Handler
}

func NewConsumer(reader MessageReader, handler middleware.Handler) *Consumer {
	return &Consumer{reader: reader, handler: handler}
}

// Run repeats: get one message, handle it, save progress if successful.
// It finishes each message before asking for the next, even when assigned several
// partitions. This keeps the example easy to follow and avoids saving progress
// past unfinished work. A larger fetch setting does not make handlers run in parallel.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		// Step 1: wait for a message without marking it as handled. The reader
		// briefly holds up partition moves until handle finishes this attempt.
		message, err := c.reader.FetchRecord(ctx)
		if err != nil {
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			return fmt.Errorf("Kafka consumer fetch: %w", err)
		}
		if err := c.handle(ctx, message); err != nil {
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			// Stop the consumer and report where handling failed. Do not save progress
			// for this message. Restarting the same group can read it again, so a bad
			// message will keep failing until it is fixed or given a recovery policy.
			return fmt.Errorf("Kafka consumer topic=%s partition=%d offset=%d: %w", message.Topic, message.Partition, message.Offset, err)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, message *kgo.Record) error {
	// Step 2: arrange to let partitions move again when this attempt finishes.
	// "defer" runs the call when this function returns, including on errors.
	// AllowRebalance does not save progress; only CommitRecords can advance
	// the group's saved position.
	defer c.reader.AllowRebalance()
	// The injected chain decodes, observes, and handles this record.
	if err := c.handler(ctx, message); err != nil {
		return err
	}
	// Handling succeeded, so ask Kafka to save the group's next position.
	// Example: after handling offset 7, commit saves 8 as the next message to read.
	// If we crash after handling but before this commit succeeds, Kafka can give
	// us offset 7 again. This possibility is called "at-least-once processing".
	// Before adding real sending, we must decide how to avoid sending duplicates.
	// Cancellation can interrupt the commit too; do not assume progress was saved.
	if err := c.reader.CommitRecords(ctx, message); err != nil {
		return fmt.Errorf("commit record: %w", err)
	}
	return nil
}
