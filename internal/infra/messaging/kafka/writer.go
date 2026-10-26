package kafka

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Writer sends messages to one Kafka topic using franz-go.
// franz-go calls a message a "record"; kgo is the name of its Go package.
//
// The process is:
//  1. NewWriter prepares a client with our settings.
//  2. WriteRecords gives messages to that client and waits for their results.
//  3. Close stops the client's background work and closes its connections.
//
// Creating the client does not prove Kafka is reachable. Connection problems
// can first appear when publishing. Kafka-specific types stay out of usecases.
type Writer struct {
	client          *kgo.Client
	topic           string
	deliveryTimeout time.Duration
	closed          atomic.Bool
	closeOnce       sync.Once
}

func NewWriter(cfg config.KafkaPublisherConfig) (*Writer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("Kafka writer configuration: %w", err)
	}
	// Choose how each message gets its partition:
	// - round-robin: take turns across partitions, even if messages have keys;
	// - key: calculate a partition from the key, so the same key stays together
	//   while the topic's partition count is unchanged.
	// Our keyPartitioner keeps the previous writer's calculation. Switching to
	// franz-go's default calculation could send an existing key elsewhere.
	var partitioner kgo.Partitioner = kgo.RoundRobinPartitioner()
	if cfg.PartitionStrategy == config.PartitionByKey {
		partitioner = keyPartitioner{}
	}
	acks := kgo.AllISRAcks()
	switch cfg.RequiredAcks {
	case config.KafkaAcksLeader:
		acks = kgo.LeaderAck()
	case config.KafkaAcksNone:
		acks = kgo.NoAck()
	}
	codec := kgo.NoCompression()
	switch cfg.Compression {
	case config.CompressionGzip:
		codec = kgo.GzipCompression()
	case config.CompressionSnappy:
		codec = kgo.SnappyCompression()
	case config.CompressionLz4:
		codec = kgo.Lz4Compression()
	case config.CompressionZstd:
		codec = kgo.ZstdCompression()
	}
	opts := []kgo.Opt{
		// Initial addresses used to find the Kafka cluster. The client then learns
		// which broker leads each partition and sends messages to that broker.
		// Listing three addresses does not send three copies of each message.
		kgo.SeedBrokers(cfg.Brokers...),
		// The topic used when a record does not specify one. WriteRecords below
		// also checks that callers do not try to publish to a different topic.
		kgo.DefaultProduceTopic(cfg.Topic),
		// Apply the partition choice described above.
		kgo.RecordPartitioner(partitioner),
		// Decide what reply counts as a successful publish:
		// - all: the partition leader waits for every copy currently keeping up;
		// - leader: the leader confirms its write without waiting for other copies;
		// - none: ask for no reply confirming the write.
		// Kafka calls the copies currently keeping up the "in-sync replicas" (ISR).
		// Topic settings decide how many copies exist and the minimum ISR needed
		// for all acknowledgments. On our one-broker setup, all still means one copy.
		// With none, a successful return cannot confirm Kafka accepted the message.
		// None of these replies means a consumer has processed the message.
		kgo.RequiredAcks(acks),
		// Time allowed for network communication with Kafka. For an ordinary
		// request, this is its network timeout. If a request asks Kafka to wait,
		// franz-go adds that waiting time when setting the response deadline.
		// Example: a 10s produce wait plus 10s here allows about 20s for a reply.
		// This is not the timeout for the whole WriteRecords call.
		kgo.RequestTimeoutOverhead(cfg.RequestTimeout),
		// Time Kafka is asked to wait for the required acknowledgments during
		// one publish request. A failed request may be retried.
		kgo.ProduceRequestTimeout(cfg.ProduceTimeout),
		// Time a message is allowed to spend waiting to be sent or retried.
		// WriteRecords below also gives the whole call a context deadline.
		// These timeouts ask franz-go to stop; finishing an ongoing network
		// operation can take additional time, so return is not instant at expiry.
		kgo.RecordDeliveryTimeout(cfg.DeliveryTimeout),
		// A batch is a group of messages for one partition sent together.
		// Limit its size before compression. Kafka's own message-size limits
		// still apply; increasing this setting does not change the broker.
		kgo.ProducerBatchMaxBytes(int32(cfg.BatchBytes)),
		// One network request can carry batches for several partitions. Give
		// that request a separate size limit, at least as large as one batch.
		kgo.BrokerMaxWriteBytes(max(100<<20, int32(cfg.BatchBytes))),
		// "Linger" means briefly wait for more messages before sending a batch.
		// A longer wait can make batches larger, but delays quiet traffic.
		kgo.ProducerLinger(cfg.BatchTimeout),
		// Limit how many messages can wait in this client across all partitions.
		// If full, submitting another message waits for space or cancellation.
		// This is a total queue limit, not the message count of each batch.
		kgo.MaxBufferedRecords(cfg.MaxBufferedRecords),
		// Retry only errors franz-go considers temporary. Our MaxAttempts includes
		// the first try: MaxAttempts=3 becomes two additional retries here.
		// RecordRetries covers sending messages; RequestRetries covers other
		// retryable broker requests. Missing topics have a separate counter.
		// These counters are not one shared limit for the entire call; its
		// context deadline and DeliveryTimeout also limit how long it can run.
		kgo.RecordRetries(cfg.MaxAttempts - 1),
		kgo.RequestRetries(cfg.MaxAttempts - 1),
		kgo.UnknownTopicRetries(cfg.MaxAttempts - 1),
		// Pause between retries instead of repeatedly contacting a failing broker.
		// The helper below increases the allowed pause and adds some randomness.
		kgo.RetryBackoffFn(retryBackoff(cfg.WriteBackoffMin, cfg.WriteBackoffMax)),
		// Compress batches using the configured format, or leave them uncompressed.
		// Compression uses CPU to send fewer bytes. Consumers decode it themselves;
		// they do not need a matching compression setting.
		kgo.ProducerBatchCompression(codec),
		// Allow cancellation even after a publish request has been sent. Without
		// this option, franz-go may keep waiting to protect its duplicate checks.
		// Cancellation cannot undo a write Kafka already received. If the reply
		// was lost, we may not know whether Kafka stored the message. Publishing
		// it again from application code can therefore create a duplicate.
		kgo.AllowIdempotentProduceCancellation(),
		// We leave automatic topic creation disabled, which is franz-go's default.
		// Our initialization script creates topics with deliberate settings.
	}
	// "Producer idempotency" means Kafka can recognize a retry of the same write
	// from this running producer and avoid storing it twice. franz-go enables
	// this by default, and it needs all acknowledgments to work.
	// Keep it for all; turn it off when the user selects leader or none.
	// This does not identify duplicate business requests or prevent a consumer
	// from processing a message again after a crash.
	if cfg.RequiredAcks != config.KafkaAcksAll {
		opts = append(opts, kgo.DisableIdempotentWrite())
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create Kafka writer: %w", err)
	}
	return &Writer{client: client, topic: cfg.Topic, deliveryTimeout: cfg.DeliveryTimeout}, nil
}

// WriteRecords sends messages and waits until each one has a success or failure
// result. franz-go updates the records with details such as partition and offset;
// callers must wait for this method to return before reading/changing them.
//
// Sending several records is not one atomic operation: some can succeed while
// others fail. Retrying all of them can send the successful ones twice.
func (w *Writer) WriteRecords(ctx context.Context, records ...*kgo.Record) error {
	if w.closed.Load() {
		return fmt.Errorf("publish Kafka records: %w", io.ErrClosedPipe)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publish Kafka records: %w", err)
	}
	// Step 1: check the whole input before sending anything. Each record must
	// exist, and its topic must be empty (use the configured topic) or match it.
	for _, record := range records {
		if record == nil {
			return fmt.Errorf("publish Kafka records: nil record")
		}
		if record.Topic != "" && record.Topic != w.topic {
			return fmt.Errorf("publish Kafka records: record topic differs from configured topic")
		}
	}
	// Step 2: give the call its delivery deadline. An earlier caller deadline
	// still wins. Prepare a place to collect one result per record.
	ctx, cancel := context.WithTimeout(ctx, w.deliveryTimeout)
	defer cancel()
	results := make(chan error, len(records))
	// Step 3: queue each record. Produce runs the supplied function when that
	// record finishes; this function is called a callback. It saves the result.
	// We use Produce and wait ourselves so BatchTimeout still controls batching.
	// The SDK's ProduceSync helper skips that batching wait in our pinned version.
	for _, record := range records {
		w.client.Produce(ctx, record, func(_ *kgo.Record, err error) { results <- err })
	}
	// Step 4: wait for every callback, then return any failures together.
	// With RequiredAcks=none, success still only means no local send failure
	// was reported; we did not ask Kafka to confirm acceptance.
	var result error
	for range records {
		result = errors.Join(result, <-results)
	}
	if result != nil {
		return fmt.Errorf("publish Kafka records: %w", result)
	}
	return nil
}

// Close stops the client and closes its connections. Shut down in this order:
// stop starting new WriteRecords calls, wait for existing calls, then Close.
// Closing while a call is still publishing can make its records fail.
func (w *Writer) Close() error {
	w.closed.Store(true)
	w.closeOnce.Do(w.client.Close)
	return nil
}

// keyPartitioner keeps the key-to-partition calculation used before migration.
// A nil key means "no key", so use round-robin. An empty but non-nil key still
// counts as a key and always goes through the hash calculation.
type keyPartitioner struct{}

func (keyPartitioner) ForTopic(topic string) kgo.TopicPartitioner {
	return &keyTopicPartitioner{roundRobin: kgo.RoundRobinPartitioner().ForTopic(topic)}
}

type keyTopicPartitioner struct{ roundRobin kgo.TopicPartitioner }

func (*keyTopicPartitioner) RequiresConsistency(record *kgo.Record) bool { return record.Key != nil }
func (p *keyTopicPartitioner) Partition(record *kgo.Record, n int) int {
	if record.Key == nil {
		return p.roundRobin.Partition(record, n)
	}
	hash := fnv.New32a()
	// hash.Hash.Write is documented to never fail.
	_, _ = hash.Write(record.Key)
	partition := int32(hash.Sum32()) % int32(n)
	if partition < 0 {
		partition = -partition
	}
	return int(partition)
}

// retryBackoff decides how long to wait before retrying a failed request.
// Example: with minimum=100ms and maximum=1s, the upper limit grows through
// 100ms, 200ms, 400ms, 800ms, then stays at 1s. Choose a random pause between
// the minimum and that limit, so many workers do not all retry together.
// Check the limit before doubling to avoid overflowing Go's duration number.
func retryBackoff(minimum, maximum time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		limit := minimum
		for i := 0; i < attempt && limit < maximum; i++ {
			if limit > maximum/2 {
				limit = maximum
				break
			}
			limit *= 2
		}
		return minimum + time.Duration(rand.Int64N(int64(limit-minimum)+1))
	}
}

// Example setup combinations
// ==========================
// These are examples to try and measure, not settings that fit every project.
// Fields not listed keep their configured defaults.
//
// 1. Business events, such as notification requests
//    Goal: confirm Kafka stored the message; keep one recipient's messages together.
//    Settings:
//      RequiredAcks      = all
//      PartitionStrategy = key (the publisher supplies the recipient ID as the key)
//      BatchTimeout      = 50ms
//      Compression       = snappy
//    Topic setup: replication factor 3 stores three copies per partition;
//    min.insync.replicas 2 requires at least two up-to-date copies for all acks.
//    Cost: that setup needs at least three brokers. Our one-broker Compose setup
//    cannot provide those backup copies. Consumers must still handle duplicates.
//
// 2. Messages that should be sent quickly
//    Goal: avoid a long batching wait when messages arrive one at a time.
//    Settings:
//      RequiredAcks      = all
//      PartitionStrategy = key or round-robin, depending on whether grouping matters
//      BatchTimeout      = 1ms
//      Compression       = none
//    Cost: smaller batches can mean more network requests. You can keep strong
//    acknowledgments while choosing a short batching wait.
//
// 3. Many independent analytics events
//    Goal: send more data per request and reduce network bytes.
//    Settings:
//      RequiredAcks      = all
//      PartitionStrategy = round-robin
//      BatchTimeout      = 100ms
//      Compression       = lz4
//    Cost: individual events may wait longer. Measure BatchBytes, broker limits,
//    CPU, and memory with realistic messages before increasing batch sizes.
//
// 4. Measurements where losing some messages is acceptable
//    Goal: send without waiting for Kafka to confirm each write.
//    Settings:
//      RequiredAcks      = none
//      PartitionStrategy = round-robin
//      MaxAttempts       = 1
//      BatchTimeout      = 100ms
//      Compression       = lz4
//    Cost: success cannot confirm Kafka accepted the message. Kafka cannot report
//    rejection through an acknowledgment reply because we did not request one.
//
// 5. Fan-out: one stream read by several independent workflows
//    Example: notification handling and auditing both need the same events.
//    Publisher: choose the required acknowledgment/batching settings above.
//    Consumers: use different GroupIDs, e.g. notifications-v1 and audit-v1.
//    Each group reads the stream and saves its own progress. Adding workers to
//    the SAME group instead shares the work; it does not create another full copy
//    of the workflow. These choices are made by consumers, not the partitioner.
//
// 6. Fan-in: several producers write to one topic
//    Example: several services all publish notification requests.
//    Publishers: agree on the message format and, if grouping matters, the same
//    key calculation. Our key strategy preserves the previous writer's mapping.
//    Consumers: read the resulting stream. Combining several related events into
//    one result requires application logic; sharing a topic does not do that.
//    Kafka has no single message order across different partitions.
//
// Rules shared by these examples
// ------------------------------
//   - WriteRecords waits for results; the client still batches messages internally.
//   - Create topics before publishing; a typo must not silently create a new topic.
//   - Producer success does not mean a consumer processed the message.
//   - A canceled publish might already have reached Kafka. Retrying it can duplicate it.
//   - Stop new writes, wait for current calls, then close the writer.
