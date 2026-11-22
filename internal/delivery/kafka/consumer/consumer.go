package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
)

// MessageReader supplies records and commits partition progress for a regular
// consumer group. FetchRecord holds up partition reassignment until AllowRebalance;
// committing is a separate operation and happens only after successful handling.
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

// Run fetches, handles, and commits one record at a time, even across partitions.
// This preserves partition order and avoids committing past unfinished work.
// A fetch, handler, or commit failure stops the consumer; parent cancellation is
// normal shutdown. The app closes the reader after Run returns.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		// Polling does not commit progress. The rebalance hold lasts through handle.
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
	// Always let partitions move after this attempt, including on failure.
	// AllowRebalance does not commit progress.
	defer c.reader.AllowRebalance()
	// The injected chain decodes, observes, and handles this record.
	if err := c.handler(ctx, message); err != nil {
		return err
	}
	// Commit offset + 1 as the next position. A crash or failed commit can replay
	// already handled work: this is at-least-once processing, so real side effects
	// must tolerate duplicates. Cancellation does not prove the commit failed.
	if err := c.reader.CommitRecords(ctx, message); err != nil {
		return fmt.Errorf("commit record: %w", err)
	}
	return nil
}
