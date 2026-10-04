package config

import (
	"slices"
	"testing"
)

// TestLoadKafkaConsumerIsIndependent protects standalone consumers from invalid publisher policies and runtime values.
func TestLoadKafkaConsumerIsIndependent(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", " localhost:9092 , localhost:19092 ")
	t.Setenv("KAFKA_TOPIC", " events.v1 ")
	t.Setenv("KAFKA_GROUP_ID", " processing-v1 ")
	t.Setenv("KAFKA_PARTITION_STRATEGY", "unknown")
	t.Setenv("KAFKA_READ_TIMEOUT", "invalid")
	t.Setenv("KAFKA_WRITE_TIMEOUT", "invalid")
	t.Setenv("DB_URL", "")
	cfg, err := LoadKafkaConsumer()
	if err != nil {
		t.Fatalf("LoadKafkaConsumer() error = %v", err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092", "localhost:19092"}) || cfg.Topic != "events.v1" || cfg.GroupID != "processing-v1" {
		t.Fatalf("consumer settings = %+v, want normalized subscription settings", cfg)
	}
}
