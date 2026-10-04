package config

import (
	"fmt"
	"strings"
	"time"
)

// PartitionStrategy describes routing without exposing a Kafka client library type.
type PartitionStrategy string

const (
	PartitionByKey      PartitionStrategy = "key"
	PartitionRoundRobin PartitionStrategy = "round-robin"
)

type KafkaPublisherConfig struct {
	Brokers           []string
	Topic             string
	PartitionStrategy PartitionStrategy
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
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
	if cfg.ReadTimeout <= 0 {
		return fmt.Errorf("KAFKA_READ_TIMEOUT must be > 0")
	}
	if cfg.WriteTimeout <= 0 {
		return fmt.Errorf("KAFKA_WRITE_TIMEOUT must be > 0")
	}
	return nil
}

// LoadKafkaPublisher reads publication settings without requiring runtime or consumer configuration.
func LoadKafkaPublisher() (KafkaPublisherConfig, error) {
	readTimeout, err := kafkaTimeout("KAFKA_READ_TIMEOUT")
	if err != nil {
		return KafkaPublisherConfig{}, err
	}
	writeTimeout, err := kafkaTimeout("KAFKA_WRITE_TIMEOUT")
	if err != nil {
		return KafkaPublisherConfig{}, err
	}
	cfg := KafkaPublisherConfig{
		Brokers:           getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:             strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
		PartitionStrategy: PartitionStrategy(strings.TrimSpace(getEnv("KAFKA_PARTITION_STRATEGY", string(PartitionRoundRobin)))),
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
	}
	if err := cfg.Validate(); err != nil {
		return KafkaPublisherConfig{}, err
	}
	return cfg, nil
}

// Parse strictly so a typo cannot silently replace an intended network deadline.
func kafkaTimeout(key string) (time.Duration, error) {
	value, err := time.ParseDuration(strings.TrimSpace(getEnv(key, "10s")))
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be > 0", key)
	}
	return value, nil
}
