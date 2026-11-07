package config

import (
	"fmt"
	"strings"
	"time"
)

// KafkaWorkerConfig combines shared runtime settings with the consumer profile.
type KafkaWorkerConfig struct {
	RuntimeConfig
	Kafka KafkaConsumerConfig
	// Cooperative limit for one processing attempt; handlers must honor context.
	// Leave enough rebalance time for processing and the subsequent commit.
	HandlingTimeout time.Duration
}

func LoadKafkaWorker() (KafkaWorkerConfig, error) {
	runtimeConfig, err := LoadRuntime()
	if err != nil {
		return KafkaWorkerConfig{}, err
	}
	consumerConfig, err := LoadKafkaConsumer()
	if err != nil {
		return KafkaWorkerConfig{}, err
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(getEnv("KAFKA_HANDLING_TIMEOUT", "10s")))
	if err != nil || timeout <= 0 {
		return KafkaWorkerConfig{}, fmt.Errorf("KAFKA_HANDLING_TIMEOUT must be a positive duration")
	}
	return KafkaWorkerConfig{RuntimeConfig: runtimeConfig, Kafka: consumerConfig, HandlingTimeout: timeout}, nil
}
