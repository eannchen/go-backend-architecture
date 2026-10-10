package kafkaclient

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/segmentio/kafka-go"
)

// Reader owns a group subscription and exposes explicit fetching and committing.
// SDK message types must not cross into usecase or repository contracts.
// ReadMessage is deliberately absent because it commits automatically in group mode.
type Reader struct {
	reader *kafka.Reader
	closed atomic.Bool
}

// NewReader starts the SDK's group lifecycle; construction does not prove broker
// connectivity. The app must Close it to stop background work and leave the group.
func NewReader(cfg config.KafkaConsumerConfig) (*Reader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("Kafka reader configuration: %w", err)
	}
	startOffset := kafka.FirstOffset
	if cfg.StartOffset == config.KafkaStartLatest {
		startOffset = kafka.LastOffset
	}
	readerConfig := kafka.ReaderConfig{
		// Discovery addresses for one cluster; the reader follows assigned leaders.
		Brokers: cfg.Brokers,
		Topic:   cfg.Topic,
		// Instances in this group share partitions. This balancing is separate from
		// the writer's key/round-robin message routing; broker payloads do not select handlers.
		GroupID: cfg.GroupID,
		// A larger minimum can delay sparse traffic while the broker accumulates data.
		MinBytes: cfg.FetchMinBytes,
		// Fetch-size and queue bounds are distinct; neither is a total memory budget.
		MaxBytes: cfg.FetchMaxBytes,
		// Broker-side wait for data, not the caller's overall deadline.
		MaxWait:          cfg.FetchMaxWait,
		ReadBatchTimeout: cfg.ReadBatchTimeout,
		QueueCapacity:    cfg.QueueCapacity,
		// Applied only when a partition has no committed offset for this group.
		StartOffset: startOffset,
		// CommitMessages waits for the commit result instead of enqueueing a periodic
		// commit. This alone does not prevent auto-commits; using FetchMessage does.
		CommitInterval: 0,
	}
	// Validate before the SDK constructor, which panics on an invalid ReaderConfig.
	if err := readerConfig.Validate(); err != nil {
		return nil, fmt.Errorf("Kafka reader SDK configuration: %w", err)
	}
	return &Reader{reader: kafka.NewReader(readerConfig)}, nil
}

// FetchMessage does not commit. Pass the caller's context to bound waiting on an
// empty topic. Cancelling one fetch does not close the SDK's group subscription.
func (r *Reader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if r.closed.Load() {
		return kafka.Message{}, fmt.Errorf("fetch Kafka message: %w", io.EOF)
	}
	message, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return kafka.Message{}, fmt.Errorf("fetch Kafka message: %w", err)
	}
	return message, nil
}

// CommitMessages advances this group's resume position. Commit only successfully
// handled records: committing a higher offset also commits all earlier offsets in
// that partition. Parallel handlers therefore need completion-order coordination.
// Processing may repeat after a crash between its side effect and this commit.
func (r *Reader) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	// The SDK's buffered commit channel can accept requests after its group loop
	// stops. Reject them here instead of waiting for a commit result that cannot arrive.
	if r.closed.Load() {
		return fmt.Errorf("commit Kafka messages: %w", io.ErrClosedPipe)
	}
	if err := r.reader.CommitMessages(ctx, messages...); err != nil {
		return fmt.Errorf("commit Kafka messages: %w", err)
	}
	return nil
}

// Close stops background fetch/group work and releases connections. The worker
// must stop accepting work and finish its chosen processing/commit policy first;
// closing a reader does not mark fetched messages as successfully processed.
func (r *Reader) Close() error {
	r.closed.Store(true)
	if err := r.reader.Close(); err != nil {
		return fmt.Errorf("close Kafka reader: %w", err)
	}
	return nil
}

// Example setup combinations
// ==========================
// Starting points to measure, not universal presets. Settings not listed below
// retain configured defaults. Pair these with producer examples in writer.go;
// producer acknowledgments and consumer commits represent different progress.
//
// 1. Shared job workers
//    Goal: distribute independent jobs among interchangeable worker instances.
//    Subscription:
//      Topic             = jobs.v1
//      GroupID           = job-workers-v1 (same for every instance)
//      FetchMinBytes     = 1
//      QueueCapacity     = 100
//    Processing: route the payload's job type to a handler, then commit on success.
//    Tradeoff: every instance must support the jobs it may receive. Kafka assigns
//    partitions, not payload types; specialised handlers do not select delivery.
//
// 2. Per-entity ordered processing
//    Goal: process related events in partition order.
//    Subscription: use one group for the workflow; producer uses compatible key routing.
//    Processing: finish each partition's records sequentially before advancing its commit.
//    Tradeoff: a busy key can limit throughput. Concurrent handlers can finish out
//    of order even though Kafka delivers each partition's records in log order.
//
// 3. Low-latency consumption
//    Goal: receive available records without waiting for a large fetch response.
//    Subscription:
//      FetchMinBytes     = 1
//      FetchMaxWait      = 500ms
//      QueueCapacity     = 100
//    Processing: keep handler and commit latency within the application's budget.
//    Tradeoff: smaller responses can mean more requests. MaxWait is a maximum
//    broker wait for enough data, not a fixed delay or the caller's fetch deadline.
//
// 4. Throughput-oriented analytics
//    Goal: fetch larger responses and process bounded batches.
//    Subscription:
//      FetchMinBytes     = 65536
//      FetchMaxBytes     = 4000000
//      FetchMaxWait      = 1s
//      QueueCapacity     = 1000
//    Processing: bound batch count/time and commit only after successful processing.
//    Tradeoff: increased latency and buffering. Queue count is not a byte budget;
//    match fetch sizes to topic records and measure memory with real payloads.
//
// 5. Fan-out to independent workflows
//    Goal: each workflow independently consumes the same retained stream.
//    Subscription:
//      Topic             = events.v1 (shared)
//      GroupID           = email-v1 / audit-v1 (different per workflow)
//    Processing: each group owns its offsets. If it intentionally ignores unrelated
//    payload types, it still advances past them; relevant failures need a policy.
//    Tradeoff: each group reads the stream. Payload filtering happens in the app,
//    not at the broker. Instances of one workflow share that workflow's group ID.
//
// 6. Replay or consume only new events
//    Goal: choose the starting position for a group with no committed offsets.
//    Subscription:
//      StartOffset       = earliest (oldest retained records) or latest (new records)
//      GroupID           = a new group for an independent replay
//    Tradeoff: existing group commits take precedence. Changing StartOffset does
//    not reset progress; replay can repeat side effects and needs an explicit policy.
//
// Shared lifecycle and delivery considerations
// --------------------------------------------
//   - This reader uses one topic and a consumer group; direct partition reading
//     and multi-topic subscriptions are separate extensions if a workflow needs them.
//   - FetchMessage does not commit. CommitMessages is synchronous; callers may
//     batch commits, but higher offsets also advance past earlier records in a partition.
//   - Commit only after successful processing. A crash before commit can repeat
//     processing, so side effects need deduplication or another idempotency design.
//   - Bound parallel processing and coordinate completion before committing.
//     Fetch/queue settings do not configure application worker concurrency.
//   - Caller contexts bound fetch/commit waits. Stop fetching, finish the chosen
//     processing/commit policy, then Close the subscription during shutdown.
