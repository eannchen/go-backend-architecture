package config

import (
	"fmt"
	"strings"
)

// Both publishers and consumers need cluster discovery addresses and a topic.
func validateKafkaConnection(brokers []string, topic string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS must contain at least one broker")
	}
	for i, broker := range brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("KAFKA_BROKERS entry at index %d must not be empty", i)
		}
	}
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("KAFKA_TOPIC must not be empty")
	}
	return nil
}
