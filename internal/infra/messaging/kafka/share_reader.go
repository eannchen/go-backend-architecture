package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
)

var (
	ErrShareBatchPending       = errors.New("share batch still has undecided records")
	ErrShareAcknowledgment     = errors.New("share acknowledgment failed")
	ErrShareRecordNotInBatch   = errors.New("record does not belong to the active share batch")
	ErrShareDecisionAlreadySet = errors.New("share record already has a different decision")
)

// ShareAcknowledgmentErrorHandler receives asynchronous, per-partition failures.
// The SDK calls it in the background, independently of message processing.
// Return nil to continue, or an error to request worker shutdown through FatalErrors.
// Report continuing failures here; let the lifecycle log a returned stop error.
// Keep the handler quick and never call reader methods from it.
type ShareAcknowledgmentErrorHandler func(topic string, partition int32, err error) error

// ShareReader consumes a Kafka Share Group. Several readers can share records
// from the same partition. Requires Kafka 4.2+ for the strict batch-size limit.
//
// One goroutine owns fetching, outcome decisions, and Close. Handlers may run
// in parallel, but return outcomes to that owner.
// Cancel the polling context and wait for processing to stop before Close.
//
// Default (explicit) flow: fetch -> process -> mark every outcome -> fetch again.
// Implicit mode lets the next poll accept any records left undecided. Like Java's
// KafkaShareConsumer, we offer both policies; franz-go itself always permits
// next-poll auto-acceptance, so our wrapper adds the explicit-mode guard.
// Kafka acknowledgment results arrive separately through the injected handler.
// Only shutdown waits for the pending acknowledgment queue to drain.
type ShareReader struct {
	// Owned SDK client and fixed maximum number of records per poll.
	client             *kgo.Client
	maxPollRecords     int
	groupID            string
	acknowledgmentMode config.KafkaShareAcknowledgmentMode
	// Only the owner accesses the current batch and shutdown state.
	active   *shareBatch
	closed   bool
	closeErr error
	// The callback owns error reporting. Only its first stop request is queued;
	// the buffered channel keeps the SDK callback from waiting for the worker.
	onAcknowledgmentError ShareAcknowledgmentErrorHandler
	fatalErrors           chan error
	fatalOnce             sync.Once
}

// shareBatch tracks one fetch inside ShareReader. Zero means no decision yet.
// Only the reader's owner accesses this state; handlers return outcomes to it.
// Keeping record pointers as keys also rejects late outcomes from older fetches.
// These references let ReleaseRemaining recover even if the caller loses its copy.
type shareBatch struct {
	decisions map[*kgo.Record]kgo.AckStatus
	pending   int
}

func newShareBatch(records []*kgo.Record) *shareBatch {
	decisions := make(map[*kgo.Record]kgo.AckStatus, len(records))
	for _, record := range records {
		decisions[record] = 0
	}
	return &shareBatch{decisions: decisions, pending: len(records)}
}

// NewShareReader validates settings and prepares a client. Like the regular
// reader, construction does not prove that Kafka is reachable or compatible.
// onAcknowledgmentError is required; pass an explicit handler that returns nil
// when continuing is intended.
// tlsConfig is loaded by app wiring and must be non-nil when cfg.Security.TLS is enabled.
func NewShareReader(cfg config.KafkaShareConsumerConfig, tlsConfig *tls.Config, onAcknowledgmentError ShareAcknowledgmentErrorHandler, hooks ...kgo.Hook) (*ShareReader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("Kafka share reader configuration: %w", err)
	}
	if onAcknowledgmentError == nil {
		return nil, errors.New("Kafka share reader requires an acknowledgment error handler")
	}
	security, err := securityOptions(cfg.Security, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("Kafka client security: %w", err)
	}

	r := &ShareReader{
		maxPollRecords:        cfg.MaxPollRecords,
		groupID:               cfg.GroupID,
		acknowledgmentMode:    cfg.AcknowledgmentMode,
		onAcknowledgmentError: onAcknowledgmentError,
		fatalErrors:           make(chan error, 1),
	}
	opts := []kgo.Opt{
		// Kotel hooks observe network and record activity for this client.
		kgo.WithHooks(hooks...),
		kgo.SeedBrokers(cfg.Brokers...), kgo.ConsumeTopics(cfg.Topics...), kgo.ShareGroup(cfg.GroupID),
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
		// Results arrive independently of polling and the current processing batch.
		// App wiring chooses whether each failure is reported or stops this worker.
		kgo.ShareAckCallback(r.recordAckResults),
		// Do not add DisableAutoCommit or BlockRebalanceOnPoll: those belong to
		// consumer groups, not Share Groups. Topics are provisioned separately.
	}
	opts = append(opts, security...)
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create Kafka share reader: %w", err)
	}
	r.client = client
	return r, nil
}

// FetchRecords waits for a bounded set of records. In explicit mode, every
// previously fetched record needs an Accept, Release, or Reject decision first.
// In implicit mode, polling again lets franz-go auto-accept undecided records.
// Finish handlers before the next poll in either mode.
// Treat records as read-only; use the reader methods instead of Record.Ack.
// After handlers stop, ReleaseRemaining can abandon undecided work safely.
// Holding records does not extend their Kafka locks. Finish well before each
// record's AcquisitionDeadline; DeliveryCount shows its delivery attempt count.
// Queuing decisions is enough to poll again; broker results are asynchronous.
func (r *ShareReader) FetchRecords(ctx context.Context) ([]*kgo.Record, error) {
	if r.closed {
		return nil, fmt.Errorf("fetch share batch: %w", io.EOF)
	}
	if r.acknowledgmentMode == config.KafkaShareAcknowledgmentExplicit && r.active != nil && r.active.pending > 0 {
		return nil, ErrShareBatchPending
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fetch share batch: %w", err)
	}
	// In implicit mode the SDK finalizes undecided records when polling starts,
	// even if the poll later times out. They must no longer be locally releasable.
	r.active = nil
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
		r.active = newShareBatch(records)
		return records, nil
	}
}

// Accept queues successful processing for this group; it does not wait for Kafka.
// The injected handler reports broker failures separately. Only the reader's
// owner calls this method.
func (r *ShareReader) Accept(record *kgo.Record) error {
	return r.markRecord(record, kgo.AckAccept)
}

// Release queues another delivery attempt without waiting for Kafka.
// Only the reader's owner calls it.
func (r *ShareReader) Release(record *kgo.Record) error {
	return r.markRecord(record, kgo.AckRelease)
}

// Reject queues an end to delivery to this group, without publishing to a DLT.
// It does not wait for Kafka. Only the reader's owner calls it.
func (r *ShareReader) Reject(record *kgo.Record) error {
	return r.markRecord(record, kgo.AckReject)
}

func (r *ShareReader) markRecord(record *kgo.Record, decision kgo.AckStatus) error {
	if r.closed {
		return fmt.Errorf("mark share decision: %w", io.ErrClosedPipe)
	}
	if r.active == nil {
		return ErrShareRecordNotInBatch
	}
	previous, exists := r.active.decisions[record]
	if !exists {
		return ErrShareRecordNotInBatch
	}
	if previous == decision {
		return nil
	}
	if previous != 0 {
		return ErrShareDecisionAlreadySet
	}
	// Queue the SDK acknowledgment before recording the local outcome.
	r.client.MarkAcks(decision, record)
	r.active.decisions[record] = decision
	r.active.pending--
	return nil
}

// ReleaseRemaining queues release for undecided records in the active batch.
// Use it to abandon unfinished work, including when the caller has lost its
// record references: the reader still holds them. In explicit mode this clears
// the next-poll guard. Lock expiry alone does not clear that local guard.
// The owner calls it after handlers stop. Existing decisions are preserved.
// Call before polling again; implicit acceptance cannot be undone afterward.
// Results arrive asynchronously. With no active batch it does nothing.
func (r *ShareReader) ReleaseRemaining() error {
	if r.closed {
		return fmt.Errorf("release remaining share records: %w", io.ErrClosedPipe)
	}
	if r.active == nil {
		return nil
	}
	for record, decision := range r.active.decisions {
		if decision == 0 {
			if err := r.Release(record); err != nil {
				return fmt.Errorf("release remaining share records: %w", err)
			}
		}
	}
	return nil
}

// FatalErrors reports the first acknowledgment failure the injected handler
// chose to stop for. The worker observes it independently of batch results.
// The channel stays open: callbacks may finish after a timed-out Close, so
// callers stop waiting when their own context ends.
func (r *ShareReader) FatalErrors() <-chan error { return r.fatalErrors }

func (r *ShareReader) recordAckResults(_ *kgo.Client, results kgo.ShareAckResults) {
	for _, result := range results {
		if result.Err == nil {
			continue
		}
		if err := r.onAcknowledgmentError(result.Topic, result.Partition, result.Err); err != nil {
			r.fatalOnce.Do(func() {
				r.fatalErrors <- fmt.Errorf("%w: group=%s topic=%s partition=%d: %w", ErrShareAcknowledgment, r.groupID, result.Topic, result.Partition, err)
			})
		}
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
	// Leaving sends queued decisions and releases unfinished records, so a
	// separate flush before leaving is unnecessary.
	leaveErr := r.client.LeaveGroupContext(ctx)
	// Wait for callbacks for both the queued decisions and final releases.
	// Leaving can finish before these callbacks; keep the SDK open for this wait.
	flushErr := r.client.FlushAcks(ctx)
	// Always release SDK resources, even if an earlier step failed or timed out.
	r.client.Close()
	r.active = nil
	// If shutdown began before the worker observed a stop request, return it
	// here. A failure already consumed by Run is not reported a second time.
	var fatalErr error
	select {
	case fatalErr = <-r.fatalErrors:
	default:
	}
	r.closeErr = errors.Join(leaveErr, flushErr, fatalErr)
	return r.closeErr
}
