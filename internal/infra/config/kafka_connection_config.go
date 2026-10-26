package config

import (
	"fmt"
	"os"
	"strings"
	"time"
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

// Fail visibly on removed settings instead of accepting tuning that has no effect.
func rejectKafkaSetting(key, explanation string) error {
	if _, exists := os.LookupEnv(key); exists {
		return fmt.Errorf("%s is no longer supported: %s", key, explanation)
	}
	return nil
}

// Bound network overhead before SDK construction so startup errors identify the
// setting. This is separate from broker-side waits and overall caller deadlines.
func validateKafkaRequestTimeout(timeout time.Duration) error {
	if timeout < 100*time.Millisecond || timeout > 15*time.Minute {
		return fmt.Errorf("KAFKA_REQUEST_TIMEOUT must be between 100ms and 15m")
	}
	return nil
}
