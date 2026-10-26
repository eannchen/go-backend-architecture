package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// KafkaShareConsumerConfig is for Kafka Share Groups, where several consumers
// can handle different records from the same partition at the same time.
// These are our configuration types; they do not depend on franz-go.
type KafkaShareConsumerConfig struct {
	Brokers []string
	Topic   string
	// All share readers with this ID cooperate on the same records. Use an ID
	// different from an existing ordinary consumer group's ID.
	GroupID string
	// Maximum records returned by FetchBatch AND requested from the broker.
	// It bounds one batch of work, not how many handlers run in parallel.
	// All acquired records already have a running lock timer, so choose a small
	// batch for slow/serial handlers; waiting in your queue uses processing time.
	MaxPollRecords int
	// Response size targets, in bytes. A fetch can contain many records.
	// Compression and broker exceptions mean these are not total memory limits.
	FetchMinBytes int
	FetchMaxBytes int
	// How long Kafka may wait for FetchMinBytes before replying with what it has.
	// FetchBatch itself can keep waiting until data arrives or its context ends.
	FetchMaxWait time.Duration
	// Network time in addition to Kafka's fetch wait. A 500ms fetch wait plus
	// 10s here gives roughly 10.5s to receive that request's response.
	RequestTimeout time.Duration
	// There is no client InitialOffset, CommitInterval, or RebalanceTimeout here.
	// Share Groups use record acknowledgments instead of consumer offset commits.
	// Initial position and record-lock duration are configured on the Kafka group.
}

// LoadKafkaShareConsumer loads only the share reader's settings. It does not
// require publisher, ordinary consumer, database, Redis, or runtime settings.
func LoadKafkaShareConsumer() (KafkaShareConsumerConfig, error) {
	cfg := KafkaShareConsumerConfig{
		Brokers: getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:   strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
		GroupID: strings.TrimSpace(getEnv("KAFKA_SHARE_GROUP_ID", "shared-workers-v1")),
	}
	for _, s := range []struct {
		key, fallback string
		target        *int
	}{
		{"KAFKA_SHARE_MAX_POLL_RECORDS", "100", &cfg.MaxPollRecords},
		{"KAFKA_SHARE_FETCH_MIN_BYTES", "1", &cfg.FetchMinBytes},
		{"KAFKA_SHARE_FETCH_MAX_BYTES", "1000000", &cfg.FetchMaxBytes},
	} {
		value, err := strconv.Atoi(strings.TrimSpace(getEnv(s.key, s.fallback)))
		if err != nil {
			return KafkaShareConsumerConfig{}, fmt.Errorf("%s must be an integer: %w", s.key, err)
		}
		*s.target = value
	}
	for _, s := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"KAFKA_SHARE_FETCH_MAX_WAIT", "500ms", &cfg.FetchMaxWait},
		{"KAFKA_SHARE_REQUEST_TIMEOUT", "10s", &cfg.RequestTimeout},
	} {
		value, err := time.ParseDuration(strings.TrimSpace(getEnv(s.key, s.fallback)))
		if err != nil {
			return KafkaShareConsumerConfig{}, fmt.Errorf("%s must be a duration: %w", s.key, err)
		}
		*s.target = value
	}
	if err := cfg.Validate(); err != nil {
		return KafkaShareConsumerConfig{}, err
	}
	return cfg, nil
}

// Validate also protects callers constructing configuration directly in Go.
func (cfg KafkaShareConsumerConfig) Validate() error {
	if err := validateKafkaConnection(cfg.Brokers, cfg.Topic); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		return fmt.Errorf("KAFKA_SHARE_GROUP_ID must not be empty")
	}
	// Keep a finite batch even if a typo would otherwise request an enormous one.
	if cfg.MaxPollRecords < 1 || cfg.MaxPollRecords > 10000 {
		return fmt.Errorf("KAFKA_SHARE_MAX_POLL_RECORDS must be between 1 and 10000")
	}
	if cfg.FetchMaxBytes < 1 || cfg.FetchMaxBytes > 1<<30 {
		return fmt.Errorf("KAFKA_SHARE_FETCH_MAX_BYTES must be between 1 and 1073741824")
	}
	if cfg.FetchMinBytes < 1 || cfg.FetchMinBytes > cfg.FetchMaxBytes {
		return fmt.Errorf("KAFKA_SHARE_FETCH_MIN_BYTES must be positive and <= KAFKA_SHARE_FETCH_MAX_BYTES")
	}
	if cfg.FetchMaxWait < 10*time.Millisecond || cfg.FetchMaxWait > time.Duration(math.MaxInt32)*time.Millisecond {
		return fmt.Errorf("KAFKA_SHARE_FETCH_MAX_WAIT must be between 10ms and 2147483647ms")
	}
	if cfg.RequestTimeout < 100*time.Millisecond || cfg.RequestTimeout > 15*time.Minute {
		return fmt.Errorf("KAFKA_SHARE_REQUEST_TIMEOUT must be between 100ms and 15m")
	}
	return nil
}
