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

// ShareMessageReader fetches records and queues per-record acknowledgment decisions.
// Run owns these calls; parallel handlers return outcomes without touching the reader.
// FatalErrors is observed separately so acknowledgment failures can stop a blocked poll.
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

// ShareConsumerOptions controls handling within one fetched batch. The reader
// limits batch size. Share Groups allow parallel handling within a partition;
// this consumer does not preserve record order.
type ShareConsumerOptions struct {
	// Maximum simultaneous handlers in this process, not the number of group members.
	Concurrency int
	// Zero disables renewal. A positive interval renews undecided records in
	// the current batch, including queued records. Leave enough time for franz-go
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

// Run uses the reader exclusively and waits for all handlers before returning.
// Parent cancellation is normal shutdown; the app closes the reader afterward.
// Acknowledgment failures chosen as fatal cancel polling and handlers separately;
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
	// Jobs are bounded by batch size and results by handler concurrency. Records
	// waiting in jobs already hold broker locks, so keep batches near concurrency.
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
				// Do not start queued work after shutdown or a batch failure. Still
				// report every record so the owner can release unfinished work.
				err := batchCtx.Err()
				if err == nil {
					err = c.handler(batchCtx, record)
				}
				results <- shareOutcome{record: record, err: err}
			}
		})
	}
	// Only the sender side closes results, after every handler goroutine exits.
	// Draining until results closes also waits for all handlers, including after
	// cancellation. Handlers must honor their context; Go cannot forcibly stop them.
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
