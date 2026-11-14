package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// KafkaShareWorkerConfig reuses runtime settings, but opens only resources used
// by this process. Pool settings belong to the worker, not the Kafka client.
type KafkaShareWorkerConfig struct {
	RuntimeConfig
	FailurePolicy KafkaWorkerFailurePolicy
	Routes        KafkaWorkerRoutesConfig
	// Each entry owns one reader and one bounded handler pool.
	Readers []KafkaShareWorkerReaderConfig
}

// KafkaShareWorkerReaderConfig keeps acquisition and processing limits together.
type KafkaShareWorkerReaderConfig struct {
	Kafka KafkaShareConsumerConfig
	// Maximum simultaneous record handlers in this process.
	Concurrency int
	// Each attempt gets this much time, unless its Kafka lock expires sooner.
	HandlingTimeout time.Duration
	// Finish this much before the SDK's estimated lock deadline. This leaves
	// network time for Kafka to confirm outcomes; it does not extend the lock.
	AcknowledgmentMargin time.Duration
}

// KafkaShareConsumerConfig is for Kafka Share Groups, where several consumers
// can handle different records from the same partition at the same time.
// These are our configuration types; they do not depend on franz-go.
type KafkaShareConsumerConfig struct {
	Security KafkaSecurityConfig
	Brokers  []string
	// Topic names to subscribe to. Delivery routes each record by its topic.
	Topics []string
	// All share readers with this ID cooperate on the same records. Use an ID
	// different from an existing ordinary consumer group's ID.
	GroupID string
	// Maximum records returned by FetchRecords AND requested from the broker.
	// It bounds one batch of work, not how many handlers run in parallel.
	// All acquired records already have a running lock timer, so choose a small
	// batch for slow/serial handlers; waiting in your queue uses processing time.
	MaxPollRecords int
	// Response size targets, in bytes. A fetch can contain many records.
	// Compression and broker exceptions mean these are not total memory limits.
	FetchMinBytes int
	FetchMaxBytes int
	// How long Kafka may wait for FetchMinBytes before replying with what it has.
	// FetchRecords itself can keep waiting until data arrives or its context ends.
	FetchMaxWait time.Duration
	// Network time in addition to Kafka's fetch wait. A 500ms fetch wait plus
	// 10s here gives roughly 10.5s to receive that request's response.
	RequestTimeout time.Duration
	// There is no client InitialOffset, CommitInterval, or RebalanceTimeout here.
	// Share Groups use record acknowledgments instead of consumer offset commits.
	// Initial position and record-lock duration are configured on the Kafka group.
}

func LoadKafkaShareWorker() (KafkaShareWorkerConfig, error) {
	runtime, err := LoadRuntime()
	if err != nil {
		return KafkaShareWorkerConfig{}, err
	}
	routes, err := loadKafkaWorkerRoutes()
	if err != nil {
		return KafkaShareWorkerConfig{}, err
	}
	policy, err := loadKafkaWorkerFailurePolicy()
	if err != nil {
		return KafkaShareWorkerConfig{}, err
	}
	cfg := KafkaShareWorkerConfig{RuntimeConfig: runtime, Routes: routes, FailurePolicy: policy}
	for _, workflow := range routes.workflows() {
		if workflow.topic == "" {
			continue
		}
		reader, err := loadKafkaShareWorkerReader(workflow)
		if err != nil {
			return KafkaShareWorkerConfig{}, err
		}
		cfg.Readers = append(cfg.Readers, reader)
	}
	return cfg, nil
}

// loadKafkaShareWorkerReader loads the pool first so the default acquisition
// batch fits its capacity. An explicit batch size is parsed only once.
func loadKafkaShareWorkerReader(workflow kafkaWorkflow) (KafkaShareWorkerReaderConfig, error) {
	key := workflow.prefix + "SHARE_WORKER_CONCURRENCY"
	concurrency, err := strconv.Atoi(strings.TrimSpace(getEnv(key, "4")))
	if err != nil || concurrency < 1 || concurrency > 1000 {
		return KafkaShareWorkerReaderConfig{}, fmt.Errorf("%s must be between 1 and 1000", key)
	}
	kafka, err := loadKafkaShareConsumer([]string{workflow.topic}, workflow.prefix, workflow.defaultShareGroup, concurrency)
	if err != nil {
		return KafkaShareWorkerReaderConfig{}, err
	}
	timeout, err := kafkaPositiveDuration(workflow.prefix+"SHARE_HANDLING_TIMEOUT", "10s")
	if err != nil {
		return KafkaShareWorkerReaderConfig{}, err
	}
	margin, err := kafkaPositiveDuration(workflow.prefix+"SHARE_ACKNOWLEDGMENT_MARGIN", "3s")
	if err != nil {
		return KafkaShareWorkerReaderConfig{}, err
	}
	return KafkaShareWorkerReaderConfig{
		Kafka:                kafka,
		Concurrency:          concurrency,
		HandlingTimeout:      timeout,
		AcknowledgmentMargin: margin,
	}, nil
}

// loadKafkaShareConsumer loads client settings using the share worker's derived subscriptions.
func loadKafkaShareConsumer(topics []string, prefix, defaultGroup string, defaultMaxPollRecords int) (KafkaShareConsumerConfig, error) {
	security, err := loadKafkaSecurity(prefix)
	if err != nil {
		return KafkaShareConsumerConfig{}, err
	}
	cfg := KafkaShareConsumerConfig{
		Security: security,
		Brokers:  getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topics:   topics,
		GroupID:  strings.TrimSpace(getEnv(prefix+"SHARE_GROUP_ID", defaultGroup)),
	}
	for _, s := range []struct {
		key, fallback string
		target        *int
	}{
		{prefix + "SHARE_MAX_POLL_RECORDS", strconv.Itoa(defaultMaxPollRecords), &cfg.MaxPollRecords},
		{prefix + "SHARE_FETCH_MIN_BYTES", "1", &cfg.FetchMinBytes},
		{prefix + "SHARE_FETCH_MAX_BYTES", "1000000", &cfg.FetchMaxBytes},
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
		{prefix + "SHARE_FETCH_MAX_WAIT", "500ms", &cfg.FetchMaxWait},
		{prefix + "SHARE_REQUEST_TIMEOUT", "10s", &cfg.RequestTimeout},
	} {
		value, err := kafkaDuration(s.key, s.fallback)
		if err != nil {
			return KafkaShareConsumerConfig{}, err
		}
		*s.target = value
	}
	if err := cfg.validate(prefix); err != nil {
		return KafkaShareConsumerConfig{}, err
	}
	return cfg, nil
}

// Validate also protects callers constructing configuration directly in Go.
func (cfg KafkaShareConsumerConfig) Validate() error {
	return cfg.validate("KAFKA_")
}

func (cfg KafkaShareConsumerConfig) validate(prefix string) error {
	if err := cfg.Security.validate(prefix); err != nil {
		return err
	}
	if err := validateKafkaSubscription(cfg.Brokers, cfg.Topics); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		return fmt.Errorf("%sSHARE_GROUP_ID must not be empty", prefix)
	}
	// Keep a finite batch even if a typo would otherwise request an enormous one.
	if cfg.MaxPollRecords < 1 || cfg.MaxPollRecords > 10000 {
		return fmt.Errorf("%sSHARE_MAX_POLL_RECORDS must be between 1 and 10000", prefix)
	}
	if cfg.FetchMaxBytes < 1 || cfg.FetchMaxBytes > 1<<30 {
		return fmt.Errorf("%sSHARE_FETCH_MAX_BYTES must be between 1 and 1073741824", prefix)
	}
	if cfg.FetchMinBytes < 1 || cfg.FetchMinBytes > cfg.FetchMaxBytes {
		return fmt.Errorf("%sSHARE_FETCH_MIN_BYTES must be positive and <= FetchMaxBytes", prefix)
	}
	if cfg.FetchMaxWait < 10*time.Millisecond || cfg.FetchMaxWait > time.Duration(math.MaxInt32)*time.Millisecond {
		return fmt.Errorf("%sSHARE_FETCH_MAX_WAIT must be between 10ms and 2147483647ms", prefix)
	}
	if cfg.RequestTimeout < 100*time.Millisecond || cfg.RequestTimeout > 15*time.Minute {
		return fmt.Errorf("%sSHARE_REQUEST_TIMEOUT must be between 100ms and 15m", prefix)
	}
	return nil
}
