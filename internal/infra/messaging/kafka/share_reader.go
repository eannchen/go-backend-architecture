package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
)

var (
	ErrShareBatchPending       = errors.New("share batch still has undecided records")
	ErrShareAcknowledgment     = errors.New("share acknowledgment failed")
	ErrShareRecordNotInBatch   = errors.New("record does not belong to the active share batch")
	ErrShareDecisionAlreadySet = errors.New("share record already has a different decision")
)

// ShareAcknowledgmentErrorHandler receives acknowledgment failures through
// franz-go's background callback. A failure identifies a topic and partition, not a
// processing batch. Return nil to continue, or an error to request consumer shutdown
// through FatalErrors. Log continuing failures here; the app logs stop errors.
// Keep the callback quick. Do not call reader methods or FlushAcks from it.
type ShareAcknowledgmentErrorHandler func(topic string, partition int32, err error) error

// ShareReader consumes records as a member of a Kafka Share Group. Unlike a regular
// consumer group, several members can process records from the same partition.
// Kafka temporarily locks each delivered record; lock expiry permits redelivery.
// Requires Kafka 4.2+ for the strict batch-size limit.
//
// One goroutine owns fetching, Accept/Release/Reject decisions, renewal, and Close.
// Handlers may run in parallel, but return outcomes to that owner.
// Cancel the polling context and wait for processing to stop before Close.
//
// Default (explicit) flow: fetch -> process -> mark every outcome -> fetch again.
// Implicit mode lets the next poll accept undecided records. Java's KafkaShareConsumer
// also offers both modes; franz-go auto-accepts on the next poll, so this wrapper
// adds the explicit-mode guard. Expired locks do not clear that local guard.
// Decisions only queue acknowledgments. Broker failures arrive separately through
// the injected callback. The optional flusher requests earlier sends; Close waits
// for pending acknowledgments within its context deadline.
type ShareReader struct {
	// franz-go client and settings fixed at construction.
	client             *kgo.Client
	maxPollRecords     int
	groupID            string
	acknowledgmentMode config.KafkaShareAcknowledgmentMode
	// Only the polling goroutine accesses the batch and shutdown state.
	active   *shareBatch
	closed   bool
	closeErr error
	// Optional background flusher. It uses only the thread-safe franz-go client,
	// never the active batch. Close cancels and joins it before leaving Kafka.
	stopAckFlusher context.CancelFunc
	ackFlusherDone chan struct{}
	// The callback owns error reporting. Only its first stop request is queued;
	// the buffered channel keeps the franz-go callback from waiting for the consumer.
	onAcknowledgmentError ShareAcknowledgmentErrorHandler
	fatalErrors           chan error
	fatalOnce             sync.Once
}

// shareBatch tracks one fetch inside ShareReader. Zero means no decision yet.
// Only the reader's owner accesses this state; handlers return outcomes to it.
// Record pointers identify the fetched records and reject outcomes from older polls.
// These references let ReleaseRemaining recover if the caller loses its references.
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
		// App wiring supplies hooks, including kotel's client traces and metrics.
		kgo.WithHooks(hooks...),
		kgo.SeedBrokers(cfg.Brokers...), kgo.ConsumeTopics(cfg.Topics...), kgo.ShareGroup(cfg.GroupID),
		// Bound both broker acquisition and records returned per poll. Without strict
		// mode, Kafka may deliver a whole stored batch that exceeds the requested count.
		kgo.ShareMaxRecords(int32(cfg.MaxPollRecords)), kgo.ShareMaxRecordsStrict(),
		// 0 means one fetch while polling, with none between polls. A value of 1
		// also allows fetching ahead while handlers run. Since broker locks start
		// on acquisition, fetching ahead would waste lock time in a local buffer.
		// Strict mode also selects 0 by default; keep this choice explicit.
		kgo.MaxConcurrentFetches(0),
		// Kafka waits up to FetchMaxWait to collect FetchMinBytes. The byte limits
		// bound requested responses, not handler concurrency; the first record
		// batch may exceed them. BrokerMaxReadBytes is a separate hard response
		// limit; responses exceeding it fail rather than grow memory without bound.
		kgo.FetchMinBytes(int32(cfg.FetchMinBytes)), kgo.FetchMaxBytes(int32(cfg.FetchMaxBytes)),
		kgo.FetchMaxPartitionBytes(int32(cfg.FetchMaxBytes)),
		kgo.BrokerMaxReadBytes(max(100<<20, int32(cfg.FetchMaxBytes))),
		// RequestTimeout adds network time to Kafka's fetch wait. Neither setting
		// limits handler duration or changes the broker's record-lock duration.
		kgo.FetchMaxWait(cfg.FetchMaxWait), kgo.RequestTimeoutOverhead(cfg.RequestTimeout),
		// Results arrive independently of polling and the current processing batch.
		// App wiring chooses whether each failure is logged or stops this consumer.
		kgo.ShareAckCallback(r.recordAckResults),
		// Share Groups acknowledge individual records, rather than committing a
		// partition offset. DisableAutoCommit and BlockRebalanceOnPoll apply only
		// to regular consumer groups. Topics are provisioned separately.
	}
	opts = append(opts, security...)
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create Kafka share reader: %w", err)
	}
	r.client = client
	r.startAckFlusher(cfg.AckFlushInterval)
	return r, nil
}

// startAckFlusher optionally requests earlier acknowledgment sends. Zero keeps
// franz-go's normal background sending. Call once after constructing the client.
func (r *ShareReader) startAckFlusher(interval time.Duration) {
	if interval == 0 {
		return
	}
	// This context belongs to the reader's resource lifecycle, not a record.
	ctx, cancel := context.WithCancel(context.Background())
	r.stopAckFlusher = cancel
	r.ackFlusherDone = make(chan struct{})
	go func() {
		defer close(r.ackFlusherDone)
		runAckFlusher(ctx, interval, r.client.FlushAcks)
	}()
}

func runAckFlusher(ctx context.Context, interval time.Duration, flushAcks func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			// One call at a time. Bound the wait so a continuously busy queue
			// cannot prevent the next periodic request to send pending acks.
			waitCtx, cancel := context.WithTimeout(ctx, interval)
			// FlushAcks returns only wait cancellation/deadline errors. Ending
			// this optional wait is expected and does not cancel queued acks.
			// Actual broker failures are reported by the acknowledgment callback.
			_ = flushAcks(waitCtx)
			cancel()
		}
	}
}

// FetchRecords waits for a bounded set of records. In explicit mode, every
// previously fetched record needs an Accept, Release, or Reject decision first.
// In implicit mode, polling again lets franz-go auto-accept undecided records.
// Finish handlers before the next poll in either mode.
// Treat records as read-only; use the reader methods instead of Record.Ack.
// After handlers stop, ReleaseRemaining can abandon undecided work safely.
// Holding records does not extend their locks. AcquisitionDeadline estimates the
// original lock expiry; renewal does not update it. DeliveryCount counts attempts.
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
	// In implicit mode franz-go finalizes undecided records when polling starts,
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

// Accept queues successful completion for this group. It does not wait for Kafka;
// the injected callback reports broker failures. Only the polling goroutine calls it.
func (r *ShareReader) Accept(record *kgo.Record) error {
	return r.markRecord(record, kgo.AckAccept)
}

// Release queues a request to make the record available to this group again.
// It does not wait for Kafka. Only the polling goroutine calls it.
func (r *ShareReader) Release(record *kgo.Record) error {
	return r.markRecord(record, kgo.AckRelease)
}

// Reject queues an end to delivery to this group. It does not delete the retained
// record or publish it to a dead-letter topic. Only the polling goroutine calls it;
// broker failures are reported asynchronously.
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
	// Queue the franz-go acknowledgment before recording the local outcome.
	r.client.MarkAcks(decision, record)
	r.active.decisions[record] = decision
	r.active.pending--
	return nil
}

// RenewRemaining requests longer locks for undecided records, including queued work.
// It only queues requests; broker failures arrive through the callback. In franz-go
// v1.22.3 the background timer is 1s; AckFlushInterval can request earlier sends.
// Renewal is best-effort: it neither completes records nor guarantees ownership.
// Renewing and then completing a record can cause duplicate acknowledgments in
// franz-go; Kafka may reject them and redeliver affected records. Handlers must
// tolerate repeated delivery whether renewal is enabled or not.
func (r *ShareReader) RenewRemaining() error {
	if r.closed {
		return fmt.Errorf("renew remaining share records: %w", io.ErrClosedPipe)
	}
	if r.active == nil || r.active.pending == 0 {
		return nil
	}
	// One goroutine owns this map; franz-go's acknowledgment state is separate.
	// This filter skips our final decisions. MarkAcks atomically checks its own
	// state when queuing renewal, so a final Accept/Release/Reject cannot be
	// overwritten. Selecting a record here does not provide that protection.
	records := make([]*kgo.Record, 0, r.active.pending)
	for record, decision := range r.active.decisions {
		if decision == 0 {
			records = append(records, record)
		}
	}
	r.client.MarkAcks(kgo.AckRenew, records...)
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

// FatalErrors reports the first acknowledgment failure the injected callback
// chose to stop for. The consumer observes it separately from processing results.
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
// and acknowledgment waits; franz-go resources are closed even if it expires.
func (r *ShareReader) Close(ctx context.Context) error {
	if r.closed {
		return r.closeErr
	}
	r.closed = true
	if r.stopAckFlusher != nil {
		r.stopAckFlusher()
		<-r.ackFlusherDone
	}
	// Leaving sends queued decisions and releases unfinished records, so a
	// separate flush before leaving is unnecessary.
	leaveErr := r.client.LeaveGroupContext(ctx)
	// Wait for callbacks for both the queued decisions and final releases.
	// Leaving can finish before these callbacks; keep the client open for this wait.
	flushErr := r.client.FlushAcks(ctx)
	// Always release franz-go resources, even if an earlier step failed or timed out.
	r.client.Close()
	r.active = nil
	// If shutdown began before the consumer observed a stop request, return it
	// here. A failure already consumed by Run is not reported a second time.
	var fatalErr error
	select {
	case fatalErr = <-r.fatalErrors:
	default:
	}
	if leaveErr != nil {
		leaveErr = fmt.Errorf("leave Kafka share group: %w", leaveErr)
	}
	if flushErr != nil {
		flushErr = fmt.Errorf("drain Kafka share acknowledgments: %w", flushErr)
	}
	r.closeErr = errors.Join(leaveErr, flushErr, fatalErr)
	return r.closeErr
}
