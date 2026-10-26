package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// KafkaShareWorkerConfig reuses runtime settings, but opens only resources used
// by this process. Pool settings belong to the worker, not the Kafka client.
type KafkaShareWorkerConfig struct {
	RuntimeConfig
	Kafka KafkaShareConsumerConfig
	// Maximum simultaneous notification handlers in this process.
	Concurrency int
	// Each attempt gets this much time, unless its Kafka lock expires sooner.
	HandlingTimeout time.Duration
	// Finish this much before the SDK's estimated lock deadline. This leaves
	// network time for Kafka to confirm outcomes; it does not extend the lock.
	AcknowledgmentMargin time.Duration
}

func LoadKafkaShareWorker() (KafkaShareWorkerConfig, error) {
	runtime, err := LoadRuntime()
	if err != nil {
		return KafkaShareWorkerConfig{}, err
	}
	concurrency, err := strconv.Atoi(strings.TrimSpace(getEnv("KAFKA_SHARE_WORKER_CONCURRENCY", "4")))
	if err != nil || concurrency < 1 || concurrency > 1000 {
		return KafkaShareWorkerConfig{}, fmt.Errorf("KAFKA_SHARE_WORKER_CONCURRENCY must be between 1 and 1000")
	}
	kafka, err := LoadKafkaShareConsumer()
	if err != nil {
		return KafkaShareWorkerConfig{}, err
	}
	// Keep the client default independent. For this worker, acquire only one
	// record per handler unless the user deliberately chooses a larger batch.
	if _, explicit := os.LookupEnv("KAFKA_SHARE_MAX_POLL_RECORDS"); !explicit {
		kafka.MaxPollRecords = concurrency
	}
	cfg := KafkaShareWorkerConfig{RuntimeConfig: runtime, Kafka: kafka, Concurrency: concurrency}
	for _, setting := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"KAFKA_SHARE_HANDLING_TIMEOUT", "10s", &cfg.HandlingTimeout},
		{"KAFKA_SHARE_ACKNOWLEDGMENT_MARGIN", "3s", &cfg.AcknowledgmentMargin},
	} {
		value, err := time.ParseDuration(strings.TrimSpace(getEnv(setting.key, setting.fallback)))
		if err != nil || value <= 0 {
			return KafkaShareWorkerConfig{}, fmt.Errorf("%s must be a positive duration", setting.key)
		}
		*setting.target = value
	}
	return cfg, nil
}
