package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareMessageReader is the worker-owned contract for fetching and deciding
// record outcomes. The client owns batch bookkeeping. Only Run's owner goroutine
// calls these methods; parallel handlers return their results to it.
type ShareMessageReader interface {
	FetchRecords(context.Context) ([]*kgo.Record, error)
	Accept(*kgo.Record) error
	Release(*kgo.Record) error
	Reject(*kgo.Record) error
	FlushAcknowledgments(context.Context) error
}

// ShareWorkerOptions bounds local work. Batch size is configured on the reader.
type ShareWorkerOptions struct {
	Concurrency int
}

func (o ShareWorkerOptions) Validate() error {
	if o.Concurrency < 1 || o.Concurrency > 1000 {
		return errors.New("share worker concurrency must be between 1 and 1000")
	}
	return nil
}

type ShareWorker struct {
	reader  ShareMessageReader
	handler middleware.Handler
	options ShareWorkerOptions
}

func NewShareWorker(reader ShareMessageReader, handler middleware.Handler, options ShareWorkerOptions) (*ShareWorker, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	return &ShareWorker{reader: reader, handler: handler, options: options}, nil
}

// Run owns the reader. It finishes and acknowledges one batch before fetching
// another. Records within a batch may finish in any order, even in one partition.
func (w *ShareWorker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		records, err := w.reader.FetchRecords(ctx)
		if err != nil {
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			return fmt.Errorf("share worker fetch: %w", err)
		}
		handleErr := w.handleBatch(ctx, records)
		// Shutdown uses a fresh cleanup context in App.Shutdown. Leave queued
		// decisions for Close rather than trying to flush with a canceled context.
		var flushErr error
		if ctx.Err() == nil {
			flushErr = w.reader.FlushAcknowledgments(ctx)
		}
		if err := errors.Join(handleErr, flushErr); err != nil {
			return fmt.Errorf("share worker batch: %w", err)
		}
	}
	return nil
}

type shareOutcome struct {
	record *kgo.Record
	err    error
}

func (w *ShareWorker) handleBatch(ctx context.Context, records []*kgo.Record) error {
	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Both queues are bounded by this finite batch / configured concurrency.
	// A queued record already holds a Kafka lock, so keep batches small.
	jobs := make(chan *kgo.Record, len(records))
	results := make(chan shareOutcome, w.options.Concurrency)
	for _, record := range records {
		jobs <- record
	}
	close(jobs)
	var handlers sync.WaitGroup
	for range min(w.options.Concurrency, len(records)) {
		handlers.Go(func() {
			for record := range jobs {
				results <- shareOutcome{record: record, err: w.handler(batchCtx, record)}
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
	for result := range results {
		var markErr error
		switch {
		case errors.Is(result.err, middleware.ErrInvalidMessage):
			markErr = w.reader.Reject(result.record)
		case result.err != nil:
			markErr = w.reader.Release(result.record)
			// Cancellation caused by shutdown or another failed handler is expected.
			// A handler's own timeout is a failure and stops this worker after the batch.
			if batchCtx.Err() == nil || (!errors.Is(result.err, context.Canceled) && !errors.Is(result.err, context.DeadlineExceeded)) {
				failure = errors.Join(failure, fmt.Errorf("handle topic=%s partition=%d offset=%d: %w", result.record.Topic, result.record.Partition, result.record.Offset, result.err))
				cancel()
			}
		default:
			markErr = w.reader.Accept(result.record)
		}
		if markErr != nil {
			failure = errors.Join(failure, fmt.Errorf("mark share outcome: %w", markErr))
			cancel()
		}
	}
	return failure
}
