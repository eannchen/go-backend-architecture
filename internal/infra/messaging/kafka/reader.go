package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Reader receives messages from one topic as a member of a consumer group.
// franz-go calls a message a "record"; kgo is the name of its Go package.
//
// Use it in this order, one record at a time:
//  1. FetchRecord gets a message without saving progress.
//  2. The worker handles that message.
//  3. If handling succeeded, CommitRecords saves the group's progress in Kafka.
//  4. AllowRebalance lets Kafka move partitions between consumers if needed.
//     Call it even when handling or committing failed.
//
// A "rebalance" is a change to which consumer handles which partitions, for
// example when another worker starts or stops. We briefly hold up that change
// while handling a record. Close the reader after its processing loop stops.
type Reader struct {
	client    *kgo.Client
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func NewReader(cfg config.KafkaConsumerConfig) (*Reader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("Kafka reader configuration: %w", err)
	}
	offset := kgo.NewOffset().AtStart()
	if cfg.InitialOffset == config.KafkaInitialLatest {
		offset = kgo.NewOffset().AtEnd()
	}
	client, err := kgo.NewClient(
		// Find the Kafka cluster, choose the topic, and join the named group.
		// Workers with the same GroupID share partitions. A different GroupID
		// gets its own progress and can read the same messages independently.
		kgo.SeedBrokers(cfg.Brokers...), kgo.ConsumeTopics(cfg.Topic), kgo.ConsumerGroup(cfg.GroupID),
		// For a group with no saved progress: earliest starts at the oldest retained
		// messages; latest starts at the end and waits for new messages.
		// Existing saved progress takes priority over this choice.
		// If Kafka has deleted data needed by a saved position, stop with an error
		// rather than automatically jumping elsewhere and possibly skipping work.
		kgo.ConsumeStartOffset(offset), kgo.ConsumeResetOffset(kgo.NoResetOffset()),
		// Do not let the client save progress automatically, including on close.
		// Reading a message does not mean our usecase successfully handled it.
		// The worker calls CommitRecords only after successful handling.
		kgo.DisableAutoCommit(),
		// After polling gives us a record, ask franz-go to hold up partition
		// reassignment while we handle it and commit successful work. We must
		// explicitly call AllowRebalance afterward, including on failure.
		kgo.BlockRebalanceOnPoll(),
		// When a rebalance begins (e.g. another worker joins), Kafka allows group
		// members time to finish current work, commit, and rejoin the group.
		// This configures that rebalance time allowance. It is NOT a new timer
		// starting each time we fetch a record, and does not automatically call
		// AllowRebalance or cancel our Go handler when time runs out.
		// If a worker hold up the rebalance too long, Kafka can remove the worker from the group.
		// Kafka uses the largest configured allowance among the group's members;
		// our workers should finish comfortably within the configured timeout.
		kgo.RebalanceTimeout(cfg.RebalanceTimeout),
		// Limit broker fetch requests, not the number of records or partitions.
		// One request can fetch many records from several partitions led by the
		// same broker. A larger limit lets requests to different brokers overlap;
		// it does not force that many requests to run.
		// A request occupies a slot while waiting for a reply AND while that reply
		// has unread records buffered in the client. PollRecords(ctx, 1) below
		// returns one record and leaves any others buffered for later polls.
		// Draining that buffer frees the slot; committing is a separate operation.
		// Fetching does not remove records from Kafka or start parallel handlers.
		// Our default is 1, and our worker still handles one record at a time.
		kgo.MaxConcurrentFetches(cfg.MaxConcurrentFetches),
		// Kafka can wait to collect at least this much data before replying.
		// With 1 byte, it can reply as soon as a message is available.
		kgo.FetchMinBytes(int32(cfg.FetchMinBytes)),
		// Ask for responses up to this size, both for the whole fetch and for
		// each partition. These are byte sizes, not numbers of messages.
		// Kafka may send a larger first batch so a large message does not get
		// stuck forever. Decompressing messages can use more memory too.
		kgo.FetchMaxBytes(int32(cfg.FetchMaxBytes)),
		kgo.FetchMaxPartitionBytes(int32(cfg.FetchMaxBytes)),
		// A separate limit for reading one complete network response. It must
		// be large enough for our requested fetch size and the response headers.
		kgo.BrokerMaxReadBytes(max(100<<20, int32(cfg.FetchMaxBytes))),
		// Maximum time Kafka waits for FetchMinBytes before replying with what
		// it has. It can reply sooner; this is not a fixed delay per message.
		// On an empty topic the client keeps fetching until ctx is canceled.
		kgo.FetchMaxWait(cfg.FetchMaxWait),
		// Add time for network communication on top of Kafka's fetch wait.
		// Example: 10s fetch wait plus 10s here allows about 20s for a reply.
		kgo.RequestTimeoutOverhead(cfg.RequestTimeout),
		// Missing topics stay missing: our initialization script creates them.
		// Automatic topic creation is disabled by default in franz-go.
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka reader: %w", err)
	}
	return &Reader{client: client}, nil
}

// FetchRecord waits for one message. It does not mark the message as handled.
// Use one processing loop per reader; do not call it from parallel workers.
// After it returns a record, the caller must eventually call AllowRebalance.
func (r *Reader) FetchRecord(ctx context.Context) (*kgo.Record, error) {
	if r.closed.Load() {
		return nil, fmt.Errorf("fetch Kafka record: %w", io.EOF)
	}
	for {
		// "Poll" asks franz-go for available messages, waiting if necessary.
		// The 1 limits what is returned to our worker, even if the client already
		// fetched a larger batch from Kafka and is holding the rest in memory.
		fetches := r.client.PollRecords(ctx, 1)
		// A fetch result can contain errors for individual partitions. Check
		// them before handing out a record, and include all reported failures.
		var result error
		for _, failure := range fetches.Errors() {
			result = errors.Join(result, fmt.Errorf("topic=%s partition=%d: %w", failure.Topic, failure.Partition, failure.Err))
		}
		if result != nil {
			// No record will reach our handler, so release the rebalance hold here.
			r.client.AllowRebalance()
			return nil, fmt.Errorf("fetch Kafka record: %w", result)
		}
		// Give the worker the record, keeping the rebalance hold until the
		// worker finishes handling/committing and calls AllowRebalance.
		if iter := fetches.RecordIter(); !iter.Done() {
			return iter.Next(), nil
		}
		// No record and no error: release the hold and try polling again.
		r.client.AllowRebalance()
	}
}

// CommitRecords tells Kafka where this group should resume after a restart.
// It waits for Kafka's commit result before returning.
// Example: committing offset 7 saves 8 as the next position to read. It also
// marks earlier positions in that partition as passed, so never commit later
// work while earlier work is unfinished. Committing does not delete messages.
//
// Pass the original fetched records so franz-go also receives their partition
// history information (the "leader epoch") when it saves progress.
// If handling finishes but the process crashes before commit succeeds, the
// group can receive the same message again. Real side effects must tolerate that.
func (r *Reader) CommitRecords(ctx context.Context, records ...*kgo.Record) error {
	if r.closed.Load() {
		return fmt.Errorf("commit Kafka records: %w", io.ErrClosedPipe)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("commit Kafka records: %w", err)
	}
	for _, record := range records {
		if record == nil {
			return fmt.Errorf("commit Kafka records: nil record")
		}
	}
	if err := r.client.CommitRecords(ctx, records...); err != nil {
		return fmt.Errorf("commit Kafka records: %w", err)
	}
	return nil
}

// AllowRebalance means "we have finished this attempt; partitions may move now."
// It does not mark any message successful and does not save progress.
// The worker defers this call so it runs after success, failure, or cancellation.
func (r *Reader) AllowRebalance() { r.client.AllowRebalance() }

// Close leaves the consumer group and stops the client. Call it after the
// processing loop has stopped using the reader. It does not commit messages.
func (r *Reader) Close(ctx context.Context) error {
	r.closed.Store(true)
	r.closeOnce.Do(func() {
		// Step 1: release any remaining rebalance hold. Leaving the group also
		// changes partition assignments, so keeping the hold could block shutdown.
		r.client.AllowRebalance()
		// Step 2: tell Kafka this consumer is leaving. The shutdown context gives
		// that request its deadline; remember the error if leaving fails.
		if err := r.client.LeaveGroupContext(ctx); err != nil {
			r.closeErr = fmt.Errorf("leave Kafka group: %w", err)
		}
		// Step 3: stop the client's background work and close connections, even
		// if the leave request failed. Uncommitted messages remain replayable.
		r.client.CloseAllowingRebalance()
	})
	return r.closeErr
}

// Example setup combinations
// ==========================
// These are examples to try and measure, not settings that fit every project.
// Fields not listed keep their configured defaults. The writer examples explain
// publishing; these examples explain who reads messages and saves progress.
//
// 1. Several workers share one job stream
//    Goal: add workers to share the topic's partitions.
//    Settings:
//      Topic                = jobs.v1
//      GroupID              = job-workers-v1 (same for all instances)
//      FetchMinBytes        = 1
//      MaxConcurrentFetches = 1
//    Handling: inspect the payload, choose a handler, commit after success.
//    Every worker must support the jobs it can receive. Kafka assigns partitions;
//    it does not inspect the payload to choose a specialized worker.
//
// 2. Keep one recipient's requests in order
//    Goal: handle related requests in the order they appear in their partition.
//    Publisher: use the recipient ID as the message key with key partitioning.
//    Consumers: use the same group and finish each partition's records in order.
//    Our worker handles one record at a time. Starting parallel handlers later
//    needs care: finishing/committing a later record first can skip unfinished work.
//    A recipient with many messages can make its partition busier than others.
//
// 3. Receive available messages promptly
//    Goal: do not wait for a large amount of data before Kafka replies.
//    Settings:
//      FetchMinBytes        = 1
//      FetchMaxWait         = 500ms
//      MaxConcurrentFetches = 1
//    With MinBytes=1, Kafka can reply when a message is available; it does not
//    always wait 500ms. Handler speed and commit time also affect total latency.
//
// 4. Read larger amounts for analytics
//    Goal: reduce request count when there is enough data for larger responses.
//    Settings:
//      FetchMinBytes        = 65536
//      FetchMaxBytes        = 4000000
//      FetchMaxWait         = 1s
//      MaxConcurrentFetches = 2
//    Cost: more waiting and buffered data. These fetch settings do not turn our
//    one-record worker into a parallel or batch handler. That needs a separate,
//    bounded processing design and careful decisions about when to commit.
//
// 5. Fan-out to notification handling and auditing
//    Goal: both workflows independently read the same topic.
//    Settings:
//      Topic   = events.v1 (same topic for both workflows)
//      GroupID = notifications-v1 / audit-v1 (different for each workflow)
//    Each group saves its own progress. A group that does not need a particular
//    payload type must deliberately skip and commit it to move on. Kafka does
//    not filter records by payload. Workers within one workflow share its GroupID.
//
// 6. Read old messages again, or start with new messages only
//    Goal: choose the starting point for a group with no saved progress.
//    Settings:
//      InitialOffset = earliest (oldest retained messages) or latest (future messages)
//      GroupID     = a new name if you want an independent read of old messages
//    Changing InitialOffset does not rewind an existing group's saved progress.
//    Reading old messages again also repeats handling, so real sending needs a
//    way to recognize work that was already done.
//
// Rules shared by these examples
// ------------------------------
//   - Getting a message does not save progress. Commit only after successful handling.
//   - Saving a later offset also passes all earlier offsets in that partition.
//   - Release the rebalance hold after each attempt, even after errors.
//   - The hold is temporary; keep handling well within RebalanceTimeout.
//   - Stop the processing loop before closing the reader. Close does not commit.
