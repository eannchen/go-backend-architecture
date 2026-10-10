package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// KafkaStartOffset selects where a group begins when no committed offset exists.
type KafkaStartOffset string

const (
	KafkaStartEarliest KafkaStartOffset = "earliest"
	KafkaStartLatest   KafkaStartOffset = "latest"
)

type KafkaConsumerConfig struct {
	// These addresses discover the cluster; the group coordinator assigns partitions.
	Brokers []string
	Topic   string
	// Same group shares work; different groups independently consume the topic.
	GroupID string
	// FetchMinBytes trades delivery latency for larger broker fetch responses.
	FetchMinBytes int
	// FetchMaxBytes limits fetch batch size, not total process memory. Size it for
	// the records the topic accepts; queue capacity also affects buffered memory.
	FetchMaxBytes int
	// FetchMaxWait is the broker's wait for enough data, not a FetchMessage deadline.
	FetchMaxWait time.Duration
	// ReadBatchTimeout bounds reading messages from a fetched batch.
	ReadBatchTimeout time.Duration
	// QueueCapacity bounds the SDK's message queue by count, not bytes.
	QueueCapacity int
	// Existing committed offsets take precedence. Earliest does not replay a
	// previously consumed group; replay requires a new group or an offset reset.
	StartOffset KafkaStartOffset
}

// LoadKafkaConsumer reads a subscription without requiring publisher or runtime configuration.
func LoadKafkaConsumer() (KafkaConsumerConfig, error) {
	cfg := KafkaConsumerConfig{
		Brokers:     getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:       strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
		GroupID:     strings.TrimSpace(getEnv("KAFKA_GROUP_ID", "notification-sender-v1")),
		StartOffset: KafkaStartOffset(strings.TrimSpace(getEnv("KAFKA_START_OFFSET", string(KafkaStartEarliest)))),
	}
	// Baseline defaults match kafka-go. MinBytes=1 allows prompt delivery whenever
	// data is available; MaxWait is not an unconditional delay on each message.
	for _, setting := range []struct {
		key, fallback string
		target        *int
	}{
		{"KAFKA_FETCH_MIN_BYTES", "1", &cfg.FetchMinBytes},
		{"KAFKA_FETCH_MAX_BYTES", "1000000", &cfg.FetchMaxBytes},
		{"KAFKA_QUEUE_CAPACITY", "100", &cfg.QueueCapacity},
	} {
		value, err := strconv.Atoi(strings.TrimSpace(getEnv(setting.key, setting.fallback)))
		if err != nil {
			return KafkaConsumerConfig{}, fmt.Errorf("%s must be an integer: %w", setting.key, err)
		}
		*setting.target = value
	}
	for _, setting := range []struct {
		key    string
		target *time.Duration
	}{
		{"KAFKA_FETCH_MAX_WAIT", &cfg.FetchMaxWait},
		{"KAFKA_READ_BATCH_TIMEOUT", &cfg.ReadBatchTimeout},
	} {
		value, err := time.ParseDuration(strings.TrimSpace(getEnv(setting.key, "10s")))
		if err != nil {
			return KafkaConsumerConfig{}, fmt.Errorf("%s must be a duration: %w", setting.key, err)
		}
		*setting.target = value
	}
	if err := cfg.Validate(); err != nil {
		return KafkaConsumerConfig{}, err
	}
	return cfg, nil
}

// Validate protects direct constructors as well as environment-backed subscriptions.
func (cfg KafkaConsumerConfig) Validate() error {
	if err := validateKafkaConnection(cfg.Brokers, cfg.Topic); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		return fmt.Errorf("KAFKA_GROUP_ID must not be empty")
	}
	if cfg.FetchMinBytes <= 0 {
		return fmt.Errorf("KAFKA_FETCH_MIN_BYTES must be > 0")
	}
	if cfg.FetchMaxBytes <= 0 {
		return fmt.Errorf("KAFKA_FETCH_MAX_BYTES must be > 0")
	}
	if cfg.FetchMinBytes > cfg.FetchMaxBytes {
		return fmt.Errorf("KAFKA_FETCH_MIN_BYTES must be <= KAFKA_FETCH_MAX_BYTES")
	}
	if cfg.QueueCapacity <= 0 {
		return fmt.Errorf("KAFKA_QUEUE_CAPACITY must be > 0")
	}
	if cfg.FetchMaxWait <= 0 {
		return fmt.Errorf("KAFKA_FETCH_MAX_WAIT must be > 0")
	}
	if cfg.ReadBatchTimeout <= 0 {
		return fmt.Errorf("KAFKA_READ_BATCH_TIMEOUT must be > 0")
	}
	switch cfg.StartOffset {
	case KafkaStartEarliest, KafkaStartLatest:
	default:
		return fmt.Errorf("KAFKA_START_OFFSET must be earliest or latest, got %q", cfg.StartOffset)
	}
	return nil
}
