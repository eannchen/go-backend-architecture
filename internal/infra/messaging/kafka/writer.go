package kafka

import (
	"context"
	"fmt"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkago "github.com/segmentio/kafka-go"
)

// Writer owns a publisher and its dedicated connection pool. Configure it at
// construction; keep SDK message types inside infra and composition boundaries.
type Writer struct {
	writer    *kafkago.Writer
	transport *kafkago.Transport
}

// NewWriter constructs a lazy publisher; construction does not prove connectivity.
func NewWriter(cfg config.KafkaPublisherConfig) (*Writer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("Kafka writer configuration: %w", err)
	}

	var balancer kafkago.Balancer
	switch cfg.PartitionStrategy {
	case config.PartitionByKey:
		balancer = &kafkago.Hash{}
	case config.PartitionRoundRobin:
		balancer = &kafkago.RoundRobin{}
	}

	var requiredAcks kafkago.RequiredAcks
	switch cfg.RequiredAcks {
	case config.KafkaAcksAll:
		requiredAcks = kafkago.RequireAll
	case config.KafkaAcksLeader:
		requiredAcks = kafkago.RequireOne
	case config.KafkaAcksNone:
		requiredAcks = kafkago.RequireNone
	}

	var compression kafkago.Compression
	switch cfg.Compression {
	case config.CompressionNone:
		compression = 0
	case config.CompressionGzip:
		compression = kafkago.Gzip
	case config.CompressionSnappy:
		compression = kafkago.Snappy
	case config.CompressionLz4:
		compression = kafkago.Lz4
	case config.CompressionZstd:
		compression = kafkago.Zstd
	}

	// Transport performs Kafka request/response networking: it discovers partition
	// leaders, caches cluster metadata, and reuses connections across requests.
	// The empty value uses SDK defaults; it opens connections lazily and currently
	// has no TLS or SASL authentication configured. Each publisher owns its pool.
	transport := &kafkago.Transport{}

	writer := &kafkago.Writer{
		// Discovery addresses belong to one cluster. The transport finds the selected
		// partition's leader; it does not publish a copy to every listed address.
		Addr: kafkago.TCP(cfg.Brokers...),
		// This writer publishes to one topic. Leave Message.Topic unset because the
		// SDK rejects specifying a topic both here and on an individual message.
		Topic: cfg.Topic,
		// Choose the partition before sending: Hash groups the same key while the
		// partition set is stable; RoundRobin rotates even when messages have keys.
		// Routing does not measure processing cost or prevent application races.
		Balancer: balancer,
		// RequireNone waits for no broker response; RequireOne waits for the leader;
		// RequireAll waits for every replica currently in the in-sync set (ISR).
		// Topic replication and min.insync.replicas determine the available copies
		// and minimum acceptable ISR. This is not a consumer or deduplication guarantee.
		// With RequireNone, a nil error cannot confirm broker acceptance; broker-side
		// rejection has no acknowledgment response through which to reach the caller.
		RequiredAcks: requiredAcks,
		// False makes WriteMessages wait for delivery results or caller cancellation.
		// True returns without waiting for delivery, so errors need a Completion
		// callback. This choice is separate from RequiredAcks; both modes can batch.
		Async: false,
		// Bounds topic partition metadata lookup; it does not consume topic messages.
		ReadTimeout: cfg.ReadTimeout,
		// Bounds one produce attempt, including the required broker acknowledgment.
		// Batching and multiple attempts can make the caller's total wait longer.
		WriteTimeout: cfg.WriteTimeout,
		// Per-partition batches flush when count/bytes reach their limits or the
		// batch timer expires. Larger batches trade latency for fewer requests.
		// BatchSize=1 avoids waiting to fill a batch, even with synchronous writes.
		BatchSize: cfg.BatchSize,
		// This is the pre-compression batch limit, not the broker's message-size setting.
		BatchBytes: cfg.BatchBytes,
		// Sparse traffic can wait for this timer before a partially filled batch is sent.
		BatchTimeout: cfg.BatchTimeout,
		// Total SDK attempts, including the first; 1 disables retries. Only SDK-classified
		// retryable failures are retried. An outer loop multiplies attempts and can
		// duplicate records. Queued attempts may continue after caller cancellation.
		MaxAttempts: cfg.MaxAttempts,
		// In v0.4.51, retry delay is attempt^2 * min, capped at max (not exponential).
		WriteBackoffMin: cfg.WriteBackoffMin,
		WriteBackoffMax: cfg.WriteBackoffMax,
		// Compress each record batch to reduce network bytes at the cost of CPU.
		// Compression=0 disables it; supported consumers decode the batch codec.
		Compression: compression,
		// Missing topics fail rather than being created with broker defaults.
		// Provisioning owns partition count, replication, and minimum ISR size.
		AllowAutoTopicCreation: false,
		// Use this publisher's pool rather than the SDK's process-wide default.
		// Close must flush the writer before releasing transport connections.
		Transport: transport,
	}
	return &Writer{writer: writer, transport: transport}, nil
}

// WriteMessages waits for delivery or caller cancellation. Cancellation does not
// retract queued messages; SDK batch attempts may continue after this call returns.
func (w *Writer) WriteMessages(ctx context.Context, messages ...kafkago.Message) error {
	if err := w.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("publish Kafka messages: %w", err)
	}
	return nil
}

// Close rejects new writes and waits for queued attempts before releasing the
// connection pool. It has no context deadline; lifecycle budgets must account
// for batching, attempt timeouts, and backoff when choosing these settings.
func (w *Writer) Close() error {
	err := w.writer.Close()
	// The SDK does not close a caller-supplied Transport when its Writer closes.
	w.transport.CloseIdleConnections()
	if err != nil {
		return fmt.Errorf("close Kafka writer: %w", err)
	}
	return nil
}

// Example setup combinations
// ==========================
// Starting points to measure, not universal presets. Settings not listed below
// retain the configured defaults.
//
// 1. Business events / durable jobs
//    Goal: replication confirmation and per-entity ordering when needed.
//    Publisher:
//      RequiredAcks      = all
//      PartitionStrategy = key (when entity ordering matters)
//      BatchSize         = 100
//      BatchTimeout      = 50ms
//      Compression       = snappy
//    Topic: replication factor = 3; min.insync.replicas = 2.
//    Consumer: commit after processing; handle duplicates.
//    Tradeoff: requires a suitable multi-broker cluster for redundancy.
//
// 2. Low-latency events
//    Goal: send without waiting to fill a batch.
//    Publisher:
//      RequiredAcks      = all
//      PartitionStrategy = key or round-robin, according to ordering needs
//      BatchSize         = 1
//      Compression       = none
//    Tradeoff: more requests; weaker acknowledgments are not required for low latency.
//
// 3. Throughput-oriented analytics
//    Goal: batch independent events and reduce network traffic.
//    Publisher:
//      RequiredAcks      = all
//      PartitionStrategy = round-robin
//      BatchSize         = 1000
//      BatchTimeout      = 100ms
//      Compression       = lz4
//    Tradeoff: batching adds latency; tune BatchBytes and broker/topic limits
//    together, then benchmark with real payloads.
//
// 4. Best-effort telemetry
//    Goal: publish measurements where loss is explicitly acceptable.
//    Publisher:
//      RequiredAcks      = none
//      PartitionStrategy = round-robin
//      MaxAttempts       = 1
//      BatchSize         = 1000
//      BatchTimeout      = 100ms
//      Compression       = lz4
//    Tradeoff: successful return does not confirm broker acceptance. Broker-side
//    failures cannot be reported through acknowledgments because none were requested.
//
// 5. Fan-out
//    Goal: several independent workflows consume the same stream.
//    Publisher: choose an appropriate setup above.
//    Consumer: give each workflow its own consumer group; instances within one
//    group share partition work.
//    Tradeoff: each workflow owns its processing progress and duplicate handling.
//
// 6. Fan-in
//    Goal: multiple producers feed one topic.
//    Publisher: use compatible key routing when related records must share a partition.
//    Consumer: joining several results into one workflow requires application state.
//    Tradeoff: no global ordering across partitions is established.
//
// Shared lifecycle and delivery considerations
// --------------------------------------------
//   - All examples use this wrapper's Async=false and explicit topic provisioning.
//   - Async=true requires a separate completion/error-handling design.
//   - None of these combinations guarantees exactly-once business effects.
//   - Bound the caller's wait with a context deadline. Size attempts, timeouts,
//     and backoff for shutdown; pending SDK work can continue after cancellation.
//   - Local replication factor 1 provides no broker redundancy, regardless of acks.
