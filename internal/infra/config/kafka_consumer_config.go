package config

import (
	"fmt"
	"strings"
)

type KafkaConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

// LoadKafkaConsumer reads a subscription without requiring publisher or runtime configuration.
func LoadKafkaConsumer() (KafkaConsumerConfig, error) {
	cfg := KafkaConsumerConfig{
		Brokers: getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:   strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
		GroupID: strings.TrimSpace(getEnv("KAFKA_GROUP_ID", "notification-sender-v1")),
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
	return nil
}
