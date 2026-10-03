package config

import (
	"fmt"
	"strings"
)

// KafkaWorkerConfig combines shared runtime settings with the Kafka worker profile.
type KafkaWorkerConfig struct {
	RuntimeConfig
	Kafka KafkaConfig
}

// KafkaConfig holds broker addresses and the worker's topic and consumer group.
type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

func LoadKafkaWorker() (KafkaWorkerConfig, error) {
	runtimeConfig, err := LoadRuntime()
	if err != nil {
		return KafkaWorkerConfig{}, err
	}
	cfg := KafkaWorkerConfig{
		RuntimeConfig: runtimeConfig,
		Kafka: KafkaConfig{
			Brokers: getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
			Topic:   strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
			GroupID: strings.TrimSpace(getEnv("KAFKA_GROUP_ID", "notification-sender-v1")),
		},
	}

	if len(cfg.Kafka.Brokers) == 0 {
		return KafkaWorkerConfig{}, fmt.Errorf("KAFKA_BROKERS must contain at least one broker")
	}
	if cfg.Kafka.Topic == "" {
		return KafkaWorkerConfig{}, fmt.Errorf("KAFKA_TOPIC must not be empty")
	}
	if cfg.Kafka.GroupID == "" {
		return KafkaWorkerConfig{}, fmt.Errorf("KAFKA_GROUP_ID must not be empty")
	}

	return cfg, nil
}
