package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PartitionStrategy describes routing without exposing a Kafka client library type.
type PartitionStrategy string

const (
	PartitionByKey      PartitionStrategy = "key"
	PartitionRoundRobin PartitionStrategy = "round-robin"
)

// KafkaAcknowledgments describes the broker confirmation required for publication.
// It remains independent of the SDK's numeric acknowledgment constants.
type KafkaAcknowledgments string

const (
	KafkaAcksAll    KafkaAcknowledgments = "all"
	KafkaAcksLeader KafkaAcknowledgments = "leader"
	KafkaAcksNone   KafkaAcknowledgments = "none"
)

// KafkaCompression selects a batch codec; the infra client maps it to the SDK.
type KafkaCompression string

const (
	CompressionNone   KafkaCompression = "none"
	CompressionGzip   KafkaCompression = "gzip"
	CompressionSnappy KafkaCompression = "snappy"
	CompressionLz4    KafkaCompression = "lz4"
	CompressionZstd   KafkaCompression = "zstd"
)

type KafkaPublisherConfig struct {
	// Brokers are discovery addresses, not the topic's replica configuration.
	Brokers []string
	Topic   string
	// Key routing groups related events; round-robin ignores keys for placement.
	PartitionStrategy PartitionStrategy
	// All confirms current in-sync replication; leader confirms only the leader.
	// None requests no broker confirmation, even when publication is synchronous.
	RequiredAcks KafkaAcknowledgments
	// ReadTimeout bounds partition metadata lookup, not topic consumption.
	ReadTimeout time.Duration
	// WriteTimeout bounds one produce attempt, including its acknowledgment.
	// Caller cancellation stops waiting but may not stop queued SDK retries.
	WriteTimeout time.Duration
	// BatchSize caps messages per partition batch. Use 1 to avoid waiting to fill a batch.
	BatchSize int
	// BatchBytes caps uncompressed batch size; broker/topic size limits still apply.
	BatchBytes int64
	// BatchTimeout flushes an incomplete batch, trading latency for batching efficiency.
	BatchTimeout time.Duration
	// MaxAttempts includes the first attempt. A value of 1 disables retries.
	// Retries can duplicate records; acknowledgments alone do not provide deduplication.
	MaxAttempts int
	// Backoff bounds pace SDK retries; kafka-go v0.4.51 uses capped quadratic backoff.
	WriteBackoffMin time.Duration
	WriteBackoffMax time.Duration
	// Compression applies to batches, trading CPU for reduced network traffic.
	Compression KafkaCompression
}

// Validate rejects policies that would otherwise silently fall back to SDK defaults.
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
		{"KAFKA_READ_TIMEOUT", cfg.ReadTimeout},
		{"KAFKA_WRITE_TIMEOUT", cfg.WriteTimeout},
		{"KAFKA_BATCH_TIMEOUT", cfg.BatchTimeout},
		{"KAFKA_WRITE_BACKOFF_MIN", cfg.WriteBackoffMin},
		{"KAFKA_WRITE_BACKOFF_MAX", cfg.WriteBackoffMax},
	} {
		if setting.value <= 0 {
			return fmt.Errorf("%s must be > 0", setting.key)
		}
	}
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("KAFKA_BATCH_SIZE must be > 0")
	}
	if cfg.BatchBytes <= 0 {
		return fmt.Errorf("KAFKA_BATCH_BYTES must be > 0")
	}
	if cfg.MaxAttempts <= 0 {
		return fmt.Errorf("KAFKA_MAX_ATTEMPTS must be > 0")
	}
	if cfg.WriteBackoffMin > cfg.WriteBackoffMax {
		return fmt.Errorf("KAFKA_WRITE_BACKOFF_MIN must be <= KAFKA_WRITE_BACKOFF_MAX")
	}
	switch cfg.Compression {
	case CompressionNone, CompressionGzip, CompressionSnappy, CompressionLz4, CompressionZstd:
	default:
		return fmt.Errorf("KAFKA_COMPRESSION must be none, gzip, snappy, lz4, or zstd, got %q", cfg.Compression)
	}
	return nil
}

// LoadKafkaPublisher reads publication settings without requiring runtime or consumer configuration.
func LoadKafkaPublisher() (KafkaPublisherConfig, error) {
	cfg := KafkaPublisherConfig{
		Brokers:           getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:             strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
		PartitionStrategy: PartitionStrategy(strings.TrimSpace(getEnv("KAFKA_PARTITION_STRATEGY", string(PartitionRoundRobin)))),
		RequiredAcks:      KafkaAcknowledgments(strings.TrimSpace(getEnv("KAFKA_REQUIRED_ACKS", string(KafkaAcksAll)))),
		Compression:       KafkaCompression(strings.TrimSpace(getEnv("KAFKA_COMPRESSION", string(CompressionNone)))),
	}
	// These are kafka-go's baseline defaults, not workload-specific tuning.
	for _, setting := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"KAFKA_READ_TIMEOUT", "10s", &cfg.ReadTimeout},
		{"KAFKA_WRITE_TIMEOUT", "10s", &cfg.WriteTimeout},
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
		{"KAFKA_BATCH_SIZE", "100", &cfg.BatchSize},
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

// Parse strictly so a typo cannot silently replace an intended network deadline or retry policy.
func kafkaDuration(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(strings.TrimSpace(getEnv(key, fallback)))
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return value, nil
}
