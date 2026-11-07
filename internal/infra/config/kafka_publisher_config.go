package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// PartitionStrategy chooses how messages are spread across topic partitions.
// The config uses our own names, so it does not depend on franz-go's Go types.
type PartitionStrategy string

const (
	PartitionByKey      PartitionStrategy = "key"
	PartitionRoundRobin PartitionStrategy = "round-robin"
)

// KafkaAcknowledgments chooses which broker reply we require before reporting
// publish success. It does not say whether a consumer has handled the message.
type KafkaAcknowledgments string

const (
	KafkaAcksAll    KafkaAcknowledgments = "all"
	KafkaAcksLeader KafkaAcknowledgments = "leader"
	KafkaAcksNone   KafkaAcknowledgments = "none"
)

// KafkaCompression chooses how to compress groups of messages before sending.
// The Kafka client translates these names into franz-go settings.
type KafkaCompression string

const (
	CompressionNone   KafkaCompression = "none"
	CompressionGzip   KafkaCompression = "gzip"
	CompressionSnappy KafkaCompression = "snappy"
	CompressionLz4    KafkaCompression = "lz4"
	CompressionZstd   KafkaCompression = "zstd"
)

type KafkaPublisherConfig struct {
	// Addresses used to find the Kafka cluster, e.g. localhost:9092.
	// More addresses do not create more copies of a message.
	Brokers []string
	Topic   string
	// key: the same key goes to the same partition while partition count is stable.
	// round-robin: take turns across partitions, even when messages have keys.
	PartitionStrategy PartitionStrategy
	// all: wait for every copy currently keeping up with the partition leader.
	// leader: wait only for the leader's write. none: ask for no confirmation.
	// On our one-broker setup there is only one copy, even with all.
	RequiredAcks KafkaAcknowledgments
	// How the four time settings fit together
	// ======================================
	// Example path for one message; widths are not actual durations:
	//
	// Time --------------------------------------------------------------->
	// | queue / find broker | collect batch | request 1 | pause | retry ... |
	// |<---------------- DeliveryTimeout: whole call --------------------->|
	//                       |<BatchTimeout>|
	//
	// DeliveryTimeout covers all this work together. A retry does not get a
	// fresh delivery deadline. The call can finish early on success or failure;
	// an earlier deadline from the caller's context also takes priority.
	// BatchTimeout is the brief wait to collect messages before sending a batch.
	// It is not an extra wait that must be repeated before every retry.
	//
	// Inside ONE publish request, when acknowledgments are requested:
	//
	// Send request ------------------------------------------> receive reply
	// Broker's acknowledgment wait: |<--- ProduceTimeout --->|
	// Network response allowance:   |<-- ProduceTimeout + RequestTimeout -->|
	//
	// These are overlapping time allowances, not two mandatory waiting phases.
	// Kafka can reply immediately when the required copies confirm the write.
	// RequestTimeout adds room for network communication; it also applies to
	// requests such as finding brokers, which have no ProduceTimeout to add.
	//
	// With the defaults: collect a batch for up to 1s; a publish reply has about
	// 10s (ProduceTimeout) + 10s (RequestTimeout) of response allowance. All
	// waiting and retries share the 30s DeliveryTimeout, not 30s per request.
	// These deadlines ask the client to stop; finishing an ongoing operation
	// can take extra time. They cannot undo a message Kafka already stored.
	//
	// Allow this much time for network communication with Kafka. For requests
	// that ask Kafka to wait, franz-go adds that wait to the response deadline.
	// Example: ProduceTimeout=10s plus RequestTimeout=10s allows about 20s
	// for a publish reply. This is not the whole publication's timeout.
	RequestTimeout time.Duration
	// How long Kafka is asked to wait for acknowledgments for one publish request.
	// A failed request may be retried within the overall delivery time below.
	ProduceTimeout time.Duration
	// Limit how long publishing can take across waiting, sending, and retries.
	// The writer applies this to each WriteRecords call and to client delivery.
	// An earlier caller deadline wins. Cancellation asks the client to stop;
	// finishing an ongoing request can take additional time.
	DeliveryTimeout time.Duration
	// Maximum messages waiting in this publisher across all partitions.
	// When full, submitting more waits for space or cancellation.
	// This does not set the message count of each batch.
	MaxBufferedRecords int
	// Maximum bytes in a batch (messages for one partition sent together),
	// measured before compression. Kafka's own size limits still apply.
	BatchBytes int64
	// How long to briefly wait for more messages before sending a batch.
	// franz-go calls this "linger". Longer waits can make batches larger,
	// but also make an individual message wait longer before it is sent.
	BatchTimeout time.Duration
	// Count the first try plus retries: 3 means one try and up to two retries.
	// The client has separate retry counters for messages and other requests;
	// this is not one shared counter across the whole WriteRecords call.
	// With all acknowledgments, Kafka can recognize this producer's internal
	// retries and avoid storing them twice. A new publish from application
	// code after cancellation can still create a duplicate.
	MaxAttempts int
	// Shortest and longest retry pauses. The client gradually allows longer
	// pauses and randomizes them so several workers do not all retry together.
	WriteBackoffMin time.Duration
	WriteBackoffMax time.Duration
	// Compress batches to send fewer bytes at the cost of CPU work.
	// Consumers decode them automatically; they need no matching setting.
	Compression KafkaCompression
}

// Validate rejects invalid settings before we construct the Kafka client.
func (cfg KafkaPublisherConfig) Validate() error {
	if err := validateKafkaConnection(cfg.Brokers, cfg.Topic); err != nil {
		return err
	}
	switch cfg.PartitionStrategy {
	case PartitionByKey, PartitionRoundRobin:
	default:
		return fmt.Errorf("KAFKA_PARTITION_STRATEGY must be key or round-robin, got %q", cfg.PartitionStrategy)
	}
	switch cfg.RequiredAcks {
	case KafkaAcksAll, KafkaAcksLeader, KafkaAcksNone:
	default:
		return fmt.Errorf("KAFKA_REQUIRED_ACKS must be all, leader, or none, got %q", cfg.RequiredAcks)
	}

	for _, setting := range []struct {
		key   string
		value time.Duration
	}{
		{"KAFKA_REQUEST_TIMEOUT", cfg.RequestTimeout},
		{"KAFKA_PRODUCE_TIMEOUT", cfg.ProduceTimeout},
		{"KAFKA_DELIVERY_TIMEOUT", cfg.DeliveryTimeout},
		{"KAFKA_BATCH_TIMEOUT", cfg.BatchTimeout},
		{"KAFKA_WRITE_BACKOFF_MIN", cfg.WriteBackoffMin},
		{"KAFKA_WRITE_BACKOFF_MAX", cfg.WriteBackoffMax},
	} {
		if setting.value <= 0 {
			return fmt.Errorf("%s must be > 0", setting.key)
		}
	}
	if cfg.MaxBufferedRecords <= 0 {
		return fmt.Errorf("KAFKA_MAX_BUFFERED_RECORDS must be > 0")
	}
	if cfg.BatchBytes < 512 || cfg.BatchBytes > 1<<30 {
		return fmt.Errorf("KAFKA_BATCH_BYTES must be between 512 and 1073741824")
	}
	if cfg.MaxAttempts <= 0 {
		return fmt.Errorf("KAFKA_MAX_ATTEMPTS must be > 0")
	}
	if cfg.WriteBackoffMin > cfg.WriteBackoffMax {
		return fmt.Errorf("KAFKA_WRITE_BACKOFF_MIN must be <= KAFKA_WRITE_BACKOFF_MAX")
	}
	if err := validateKafkaRequestTimeout(cfg.RequestTimeout); err != nil {
		return err
	}
	if cfg.ProduceTimeout < 100*time.Millisecond || cfg.ProduceTimeout > time.Duration(math.MaxInt32)*time.Millisecond {
		return fmt.Errorf("KAFKA_PRODUCE_TIMEOUT must be between 100ms and 2147483647ms")
	}
	if cfg.DeliveryTimeout < time.Second {
		return fmt.Errorf("KAFKA_DELIVERY_TIMEOUT must be >= 1s")
	}
	if cfg.BatchTimeout > time.Minute {
		return fmt.Errorf("KAFKA_BATCH_TIMEOUT must be <= 1m")
	}
	switch cfg.Compression {
	case CompressionNone, CompressionGzip, CompressionSnappy, CompressionLz4, CompressionZstd:
	default:
		return fmt.Errorf("KAFKA_COMPRESSION must be none, gzip, snappy, lz4, or zstd, got %q", cfg.Compression)
	}
	return nil
}

// LoadKafkaPublisher reads only the settings needed to publish messages.
// A publisher does not need a consumer group, database, or Redis configuration.
func LoadKafkaPublisher() (KafkaPublisherConfig, error) {
	for _, setting := range [][2]string{
		{"KAFKA_READ_TIMEOUT", "use KAFKA_REQUEST_TIMEOUT (network overhead for all broker requests)"},
		{"KAFKA_WRITE_TIMEOUT", "use KAFKA_PRODUCE_TIMEOUT (broker acknowledgment wait) and KAFKA_DELIVERY_TIMEOUT (overall publication)"},
		{"KAFKA_BATCH_SIZE", "there is no per-batch record-count setting; use KAFKA_BATCH_BYTES and KAFKA_BATCH_TIMEOUT for batching, KAFKA_MAX_BUFFERED_RECORDS for total queued records"},
	} {
		if err := rejectKafkaSetting(setting[0], setting[1]); err != nil {
			return KafkaPublisherConfig{}, err
		}
	}
	cfg := KafkaPublisherConfig{
		Brokers:           getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:             strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
		PartitionStrategy: PartitionStrategy(strings.TrimSpace(getEnv("KAFKA_PARTITION_STRATEGY", string(PartitionRoundRobin)))),
		RequiredAcks:      KafkaAcknowledgments(strings.TrimSpace(getEnv("KAFKA_REQUIRED_ACKS", string(KafkaAcksAll)))),
		Compression:       KafkaCompression(strings.TrimSpace(getEnv("KAFKA_COMPRESSION", string(CompressionNone)))),
	}
	// Load the time settings. These are starting values for the template;
	// environment variables let a cloned project choose different values.
	for _, setting := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"KAFKA_REQUEST_TIMEOUT", "10s", &cfg.RequestTimeout},
		{"KAFKA_PRODUCE_TIMEOUT", "10s", &cfg.ProduceTimeout},
		{"KAFKA_DELIVERY_TIMEOUT", "30s", &cfg.DeliveryTimeout},
		{"KAFKA_BATCH_TIMEOUT", "1s", &cfg.BatchTimeout},
		{"KAFKA_WRITE_BACKOFF_MIN", "100ms", &cfg.WriteBackoffMin},
		{"KAFKA_WRITE_BACKOFF_MAX", "1s", &cfg.WriteBackoffMax},
	} {
		value, err := kafkaDuration(setting.key, setting.fallback)
		if err != nil {
			return KafkaPublisherConfig{}, err
		}
		*setting.target = value
	}
	for _, setting := range []struct {
		key, fallback string
		target        *int
	}{
		{"KAFKA_MAX_BUFFERED_RECORDS", "10000", &cfg.MaxBufferedRecords},
		{"KAFKA_MAX_ATTEMPTS", "10", &cfg.MaxAttempts},
	} {
		value, err := strconv.Atoi(strings.TrimSpace(getEnv(setting.key, setting.fallback)))
		if err != nil {
			return KafkaPublisherConfig{}, fmt.Errorf("%s must be an integer: %w", setting.key, err)
		}
		*setting.target = value
	}
	batchBytes, err := strconv.ParseInt(strings.TrimSpace(getEnv("KAFKA_BATCH_BYTES", "1048576")), 10, 64)
	if err != nil {
		return KafkaPublisherConfig{}, fmt.Errorf("KAFKA_BATCH_BYTES must be an integer: %w", err)
	}
	cfg.BatchBytes = batchBytes
	if err := cfg.Validate(); err != nil {
		return KafkaPublisherConfig{}, err
	}
	return cfg, nil
}

// A typo such as "ten seconds" must fail startup instead of using a default.
// Durations use Go notation, e.g. "500ms", "10s", or "1m".
func kafkaDuration(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(strings.TrimSpace(getEnv(key, fallback)))
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return value, nil
}
