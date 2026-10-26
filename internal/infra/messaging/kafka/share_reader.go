package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	ErrShareBatchPending   = errors.New("share batch still has undecided records")
	ErrShareAcknowledgment = errors.New("share acknowledgment failed")
)

// ShareReader consumes a Kafka Share Group. Several readers can share records
// from the same partition. Requires Kafka 4.2+ for the strict batch-size limit.
//
// One goroutine owns FetchBatch, Mark, FlushAcknowledgments, and Close.
// Handlers may run in parallel, but return their outcomes to that owner to Mark.
// Cancel the polling context and wait for processing to stop before Close.
//
// Flow: fetch -> process -> mark every outcome -> flush -> fetch again.
// FetchBatch also flushes before polling, so a forgotten flush cannot hide errors.
// This waits for acknowledgments between batches instead of pipelining them.
type ShareReader struct {
	// Owned SDK client and fixed maximum number of records per poll.
	client         *kgo.Client
	maxPollRecords int
	// Only the owner accesses the current batch and shutdown state.
	active   *ShareBatch
	closed   bool
	closeErr error
	// The SDK callback runs separately. This lock protects only its error report.
	ackMu  sync.Mutex
	ackErr error
}

// NewShareReader validates settings and prepares a client. Like the regular
// reader, construction does not prove that Kafka is reachable or compatible.
func NewShareReader(cfg config.KafkaShareConsumerConfig) (*ShareReader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("Kafka share reader configuration: %w", err)
	}
	r := &ShareReader{maxPollRecords: cfg.MaxPollRecords}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...), kgo.ConsumeTopics(cfg.Topic), kgo.ShareGroup(cfg.GroupID),
		// Ask for a strict record-count limit, then return at most that many records
		// to the caller. This makes one batch's work finite, even for large log batches.
		kgo.ShareMaxRecords(int32(cfg.MaxPollRecords)), kgo.ShareMaxRecordsStrict(),
		// 0 is a special SDK mode: allow one fetch while PollRecords is running,
		// with no background fetching between polls. It does not mean zero requests
		// or "zero extra" requests. A value of 1 allows one fetch between polls too.
		// Share Group locks start when Kafka hands out records, so fetching ahead
		// would spend their lock time while our handlers are busy with this batch.
		// ShareMaxRecordsStrict also defaults to 0; keep our choice explicit here.
		kgo.MaxConcurrentFetches(0),
		kgo.FetchMinBytes(int32(cfg.FetchMinBytes)), kgo.FetchMaxBytes(int32(cfg.FetchMaxBytes)),
		kgo.FetchMaxPartitionBytes(int32(cfg.FetchMaxBytes)),
		kgo.BrokerMaxReadBytes(max(100<<20, int32(cfg.FetchMaxBytes))),
		kgo.FetchMaxWait(cfg.FetchMaxWait), kgo.RequestTimeoutOverhead(cfg.RequestTimeout),
		// FlushAcks waits for sending to finish but does not return every broker
		// rejection. Observe the actual results here and expose failures to callers.
		kgo.ShareAckCallback(r.recordAckResults),
		// Do not add DisableAutoCommit or BlockRebalanceOnPoll: those belong to
		// ordinary consumer groups, not Share Groups. Topics are provisioned separately.
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka share reader: %w", err)
	}
	r.client = client
	return r, nil
}

// FetchBatch waits for records. Every record in the previous batch must have
// a decision: franz-go v1.22.2 would otherwise auto-accept it on the next poll.
// A flush error is returned before fetching; the owner chooses whether to retry.
func (r *ShareReader) FetchBatch(ctx context.Context) (*ShareBatch, error) {
	if r.closed {
		return nil, fmt.Errorf("fetch share batch: %w", io.EOF)
	}
	if r.active != nil && r.active.pending > 0 {
		return nil, ErrShareBatchPending
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fetch share batch: %w", err)
	}
	if err := r.FlushAcknowledgments(ctx); err != nil {
		return nil, fmt.Errorf("fetch share batch: flush previous acknowledgments before polling: %w", err)
	}
	for {
		fetches := r.client.PollRecords(ctx, r.maxPollRecords)
		records := fetches.Records()
		var fetchErr error
		for _, failure := range fetches.Errors() {
			fetchErr = errors.Join(fetchErr, fmt.Errorf("topic=%s partition=%d: %w", failure.Topic, failure.Partition, failure.Err))
		}
		if fetchErr != nil {
			// A response can contain both records and errors. Return unhandled records
			// to Kafka; do not let the next poll accept them as successful work.
			if len(records) > 0 {
				r.client.MarkAcks(kgo.AckRelease, records...)
			}
			return nil, fmt.Errorf("fetch share batch: %w", fetchErr)
		}
		if len(records) == 0 {
			continue
		}
		r.active = newShareBatch(r, records)
		return r.active, nil
	}
}

// FlushAcknowledgments waits for marked decisions and reports the first failure.
// It leaves undecided records alone. After a completed flush, the report is
// cleared; the owner chooses whether to continue or close.
//
// With franz-go v1.22.2 and Kafka 4.2.1, a lost-lock error (InvalidRecordState)
// can stall later fetches. Recreate the reader after that error.
func (r *ShareReader) FlushAcknowledgments(ctx context.Context) error {
	if r.closed {
		return fmt.Errorf("flush share acknowledgments: %w", io.ErrClosedPipe)
	}
	return r.flushAcknowledgments(ctx)
}

func (r *ShareReader) flushAcknowledgments(ctx context.Context) error {
	// FlushAcks reports whether waiting finished. Broker failures come from callbacks.
	flushErr := r.client.FlushAcks(ctx)
	r.ackMu.Lock()
	ackErr := r.ackErr
	if flushErr == nil {
		// Return the saved ackErr below, then let future flushes report NEW failures.
		// A completed wait does not mean every acknowledgment succeeded.
		r.ackErr = nil
	}
	r.ackMu.Unlock()
	// If waiting was interrupted, keep the report until a later flush completes.
	if err := errors.Join(flushErr, ackErr); err != nil {
		return fmt.Errorf("flush share acknowledgments: %w", err)
	}
	return nil
}

func (r *ShareReader) recordAckResults(_ *kgo.Client, results kgo.ShareAckResults) {
	var failure error
	for _, result := range results {
		if result.Err != nil {
			failure = errors.Join(failure, fmt.Errorf("topic=%s partition=%d: %w", result.Topic, result.Partition, result.Err))
		}
	}
	if failure == nil {
		return
	}
	r.ackMu.Lock()
	defer r.ackMu.Unlock()
	// Keep one failed result set until flush reports it, rather than accumulating
	// an unlimited error history. A later success must not erase the failure.
	if r.ackErr == nil {
		r.ackErr = errors.Join(ErrShareAcknowledgment, failure)
	}
}

// Close is called by the owner after polling and handlers have stopped.
// It releases unfinished records; it never accepts them as successful work.
// Repeated calls return the first shutdown result. The context limits network
// and acknowledgment waits; SDK resources are closed even if it expires.
func (r *ShareReader) Close(ctx context.Context) error {
	if r.closed {
		return r.closeErr
	}
	r.closed = true
	// Confirm decisions already queued by Mark before starting to leave.
	markedErr := r.flushAcknowledgments(ctx)
	// Leaving queues release acknowledgments for unfinished records.
	leaveErr := r.client.LeaveGroupContext(ctx)
	// Wait for those final results while the SDK is still open.
	finalErr := r.flushAcknowledgments(ctx)
	// Always release SDK resources, even if an earlier step failed or timed out.
	r.client.Close()
	r.active = nil
	r.closeErr = errors.Join(markedErr, leaveErr, finalErr)
	return r.closeErr
}
