package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareWorkBatch exposes the per-record decisions this delivery adapter needs.
// Only the worker's owner calls these methods; handlers return results to it.
type ShareWorkBatch interface {
	Records() []*kgo.Record
	Accept(*kgo.Record) error
	Release(*kgo.Record) error
	Reject(*kgo.Record) error
}

type ShareMessageReader interface {
	FetchBatch(context.Context) (ShareWorkBatch, error)
	FlushAcknowledgments(context.Context) error
}

// ShareWorkerOptions bounds local work. Batch size is configured on the reader.
type ShareWorkerOptions struct {
	Concurrency     int
	HandlingTimeout time.Duration
	// Leave time to send acknowledgments before Kafka's record lock expires.
	AcknowledgmentMargin time.Duration
}

func (o ShareWorkerOptions) Validate() error {
	if o.Concurrency < 1 || o.Concurrency > 1000 {
		return errors.New("share worker concurrency must be between 1 and 1000")
	}
	if o.HandlingTimeout <= 0 {
		return errors.New("share worker handling timeout must be positive")
	}
	if o.AcknowledgmentMargin <= 0 {
		return errors.New("share worker acknowledgment margin must be positive")
	}
	return nil
}

type NotificationShareWorker struct {
	reader  ShareMessageReader
	usecase notification.Usecase
	log     logger.Logger
	options ShareWorkerOptions
}

func NewNotificationShareWorker(reader ShareMessageReader, usecase notification.Usecase, log logger.Logger, options ShareWorkerOptions) (*NotificationShareWorker, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &NotificationShareWorker{reader: reader, usecase: usecase, log: log, options: options}, nil
}

// Run owns the reader. It finishes and acknowledges one batch before fetching
// another. Records within a batch may finish in any order, even in one partition.
func (w *NotificationShareWorker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		batch, err := w.reader.FetchBatch(ctx)
		if err != nil {
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			return fmt.Errorf("share notification fetch: %w", err)
		}
		handleErr := w.handleBatch(ctx, batch)
		// Shutdown uses a fresh cleanup context in App.Shutdown. Leave queued
		// decisions for Close rather than trying to flush with a canceled context.
		var flushErr error
		if ctx.Err() == nil {
			flushErr = w.reader.FlushAcknowledgments(ctx)
		}
		if err := errors.Join(handleErr, flushErr); err != nil {
			return fmt.Errorf("share notification batch: %w", err)
		}
	}
	return nil
}

type shareOutcome struct {
	record *kgo.Record
	reject bool
	err    error
}

func (w *NotificationShareWorker) handleBatch(ctx context.Context, batch ShareWorkBatch) error {
	records := batch.Records()
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
				results <- w.handleRecord(batchCtx, record)
			}
		})
	}
	var failure error
	for range len(records) {
		result := <-results
		var markErr error
		switch {
		case result.reject:
			markErr = batch.Reject(result.record)
			w.log.Warn(ctx, "invalid notification rejected for share group", logger.FromPairs(
				"kafka.topic", result.record.Topic, "kafka.partition", result.record.Partition,
				"kafka.offset", result.record.Offset, "reason", result.err.Error()))
		case result.err != nil:
			markErr = batch.Release(result.record)
			// Cancellation caused by shutdown or another failed handler is expected.
			// A handler's own timeout is a failure and stops this worker after the batch.
			if batchCtx.Err() == nil || (!errors.Is(result.err, context.Canceled) && !errors.Is(result.err, context.DeadlineExceeded)) {
				failure = errors.Join(failure, fmt.Errorf("handle topic=%s partition=%d offset=%d: %w", result.record.Topic, result.record.Partition, result.record.Offset, result.err))
				cancel()
			}
		default:
			markErr = batch.Accept(result.record)
		}
		if markErr != nil {
			failure = errors.Join(failure, fmt.Errorf("mark share outcome: %w", markErr))
			cancel()
		}
	}
	// Every handler has sent its result; join before the reader can be closed.
	handlers.Wait()
	return failure
}

func (w *NotificationShareWorker) handleRecord(ctx context.Context, record *kgo.Record) (result shareOutcome) {
	result.record = record
	defer func() {
		if recover() != nil {
			// Panic values can contain payloads. Report the failure without exposing them.
			result.err = errors.New("notification handler panicked")
			result.reject = false
		}
	}()
	deadline := time.Now().Add(w.options.HandlingTimeout)
	if lockDeadline := record.AcquisitionDeadline(); !lockDeadline.IsZero() {
		if finishBy := lockDeadline.Add(-w.options.AcknowledgmentMargin); finishBy.Before(deadline) {
			deadline = finishBy
		}
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if result.err = ctx.Err(); result.err != nil {
		return result
	}
	request, err := decodeNotification(record.Value)
	if err != nil {
		result.reject, result.err = true, err
		return result
	}
	result.err = w.usecase.Handle(ctx, request)
	// A handler must honor cancellation. Even a nil return after the deadline
	// cannot safely be treated as success: Kafka may have handed the record out again.
	if result.err == nil {
		result.err = ctx.Err()
	}
	return result
}
