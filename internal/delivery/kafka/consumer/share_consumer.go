package consumer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareMessageReader is the consumer-owned contract for fetching and deciding
// record outcomes. The client owns batch bookkeeping. Only Run's owner goroutine
// calls these methods; parallel handlers return their results to it.
type ShareMessageReader interface {
	FetchRecords(context.Context) ([]*kgo.Record, error)
	Accept(*kgo.Record) error
	Release(*kgo.Record) error
	Reject(*kgo.Record) error
	// RenewRemaining queues renewal without waiting for Kafka. Broker
	// failures are reported through the acknowledgment callback.
	RenewRemaining() error
	// FatalErrors stays open and reports independently selected stop requests.
	FatalErrors() <-chan error
}

// ShareConsumerOptions bounds local work. Batch size is configured on the reader.
type ShareConsumerOptions struct {
	Concurrency int
	// Zero disables renewal. A positive interval renews undecided records in
	// the current batch, including queued records. Leave enough time for SDK
	// background sending and network delays before the broker lock expires.
	RenewalInterval time.Duration
}

func (o ShareConsumerOptions) Validate() error {
	if o.Concurrency < 1 || o.Concurrency > 1000 {
		return errors.New("share consumer concurrency must be between 1 and 1000")
	}
	if o.RenewalInterval < 0 || (o.RenewalInterval > 0 && o.RenewalInterval < 10*time.Millisecond) || o.RenewalInterval > 15*time.Minute {
		return errors.New("share consumer renewal interval must be zero or between 10ms and 15m")
	}
	return nil
}

type ShareConsumer struct {
	reader  ShareMessageReader
	handler middleware.Handler
	options ShareConsumerOptions
}

func NewShareConsumer(reader ShareMessageReader, handler middleware.Handler, options ShareConsumerOptions) (*ShareConsumer, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	return &ShareConsumer{reader: reader, handler: handler, options: options}, nil
}

// Run owns the reader and joins all handlers before returning. Acknowledgment
// failures chosen as fatal cancel polling and handlers through a separate path;
// they are never attached to whichever batch happens to be running.
func (c *ShareConsumer) Run(ctx context.Context) (result error) {
	runCtx, cancel := context.WithCancel(ctx)
	failures := c.reader.FatalErrors()
	done := make(chan struct{})
	var acknowledgmentErr error
	go func() {
		defer close(done)
		select {
		case acknowledgmentErr = <-failures:
			cancel()
		case <-runCtx.Done():
		}
	}()
	defer func() {
		cancel()
		<-done // Join the observer before reading its error or closing the reader.
		result = errors.Join(result, acknowledgmentErr)
	}()
	for runCtx.Err() == nil {
		records, err := c.reader.FetchRecords(runCtx)
		if err != nil {
			if runCtx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			return fmt.Errorf("share consumer fetch: %w", err)
		}
		if err := c.handleBatch(runCtx, records); err != nil {
			return fmt.Errorf("share consumer batch: %w", err)
		}
	}
	return nil
}

type shareOutcome struct {
	record *kgo.Record
	err    error
}

func (c *ShareConsumer) handleBatch(ctx context.Context, records []*kgo.Record) error {
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var renewalTicks <-chan time.Time
	if c.options.RenewalInterval > 0 {
		ticker := time.NewTicker(c.options.RenewalInterval)
		defer ticker.Stop()
		renewalTicks = ticker.C
	}
	// Both queues are bounded by this finite batch / configured concurrency.
	// A queued record already holds a Kafka lock, so keep batches small.
	jobs := make(chan *kgo.Record, len(records))
	results := make(chan shareOutcome, c.options.Concurrency)
	for _, record := range records {
		jobs <- record
	}
	close(jobs)
	var handlers sync.WaitGroup
	for range min(c.options.Concurrency, len(records)) {
		handlers.Go(func() {
			for record := range jobs {
				results <- shareOutcome{record: record, err: c.handler(batchCtx, record)}
			}
		})
	}
	// Only the sender side closes results, after every handler goroutine exits.
	// The owner drains results concurrently so it can mark outcomes and cancel
	// the remaining work as soon as a failure arrives.
	go func() {
		handlers.Wait()
		close(results)
	}()
	var failure error
	for {
		select {
		case <-renewalTicks:
			if batchCtx.Err() != nil {
				renewalTicks = nil
				continue
			}
			// This queues renewal only. Remote failures are reported separately;
			// they do not become errors for the batch currently being processed.
			if err := c.reader.RenewRemaining(); err != nil {
				failure = errors.Join(failure, fmt.Errorf("queue share renewal: %w", err))
				cancel()
				renewalTicks = nil
			}
		case result, ok := <-results:
			if !ok {
				return failure
			}
			var markErr error
			switch {
			case errors.Is(result.err, middleware.ErrInvalidMessage):
				markErr = c.reader.Reject(result.record)
			case result.err != nil:
				markErr = c.reader.Release(result.record)
				// Cancellation caused by shutdown or another failed handler is expected.
				// A handler's own timeout is a failure and stops this consumer after the batch.
				if batchCtx.Err() == nil || (!errors.Is(result.err, context.Canceled) && !errors.Is(result.err, context.DeadlineExceeded)) {
					failure = errors.Join(failure, fmt.Errorf("handle topic=%s partition=%d offset=%d: %w", result.record.Topic, result.record.Partition, result.record.Offset, result.err))
					cancel()
				}
			default:
				markErr = c.reader.Accept(result.record)
			}
			if markErr != nil {
				failure = errors.Join(failure, fmt.Errorf("mark share outcome: %w", markErr))
				cancel()
			}
		}
	}
}
