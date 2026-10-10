package config

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestLoadKafkaConsumerIsIndependent protects standalone consumers from invalid publisher policies and runtime values.
func TestLoadKafkaConsumerIsIndependent(t *testing.T) {
	setKafkaConsumerFetchDefaults(t)
	t.Setenv("KAFKA_BROKERS", " localhost:9092 , localhost:19092 ")
	t.Setenv("KAFKA_TOPIC", " events.v1 ")
	t.Setenv("KAFKA_GROUP_ID", " processing-v1 ")
	t.Setenv("KAFKA_PARTITION_STRATEGY", "unknown")
	t.Setenv("KAFKA_READ_TIMEOUT", "invalid")
	t.Setenv("KAFKA_WRITE_TIMEOUT", "invalid")
	t.Setenv("KAFKA_REQUIRED_ACKS", "invalid")
	t.Setenv("KAFKA_BATCH_SIZE", "invalid")
	t.Setenv("DB_URL", "")
	cfg, err := LoadKafkaConsumer()
	if err != nil {
		t.Fatalf("LoadKafkaConsumer() error = %v", err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092", "localhost:19092"}) || cfg.Topic != "events.v1" || cfg.GroupID != "processing-v1" {
		t.Fatalf("consumer settings = %+v, want normalized subscription settings", cfg)
	}
}

// TestLoadKafkaConsumerDefaults protects a usable group subscription and explicit fetch bounds when settings are absent.
func TestLoadKafkaConsumerDefaults(t *testing.T) {
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP_ID", "KAFKA_FETCH_MIN_BYTES", "KAFKA_FETCH_MAX_BYTES", "KAFKA_FETCH_MAX_WAIT", "KAFKA_READ_BATCH_TIMEOUT", "KAFKA_QUEUE_CAPACITY", "KAFKA_START_OFFSET"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := LoadKafkaConsumer()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092"}) || cfg.Topic != "notifications.requested.v1" || cfg.GroupID != "notification-sender-v1" || cfg.FetchMinBytes != 1 || cfg.FetchMaxBytes != 1000000 || cfg.FetchMaxWait != 10*time.Second || cfg.ReadBatchTimeout != 10*time.Second || cfg.QueueCapacity != 100 || cfg.StartOffset != KafkaStartEarliest {
		t.Fatalf("unexpected consumer defaults: %+v", cfg)
	}
}

// TestLoadKafkaConsumerFetchOverrides protects normalized tuning and latest startup without depending on publisher settings.
func TestLoadKafkaConsumerFetchOverrides(t *testing.T) {
	setKafkaConsumerFetchDefaults(t)
	t.Setenv("KAFKA_BROKERS", "localhost:9092")
	t.Setenv("KAFKA_TOPIC", "events.v1")
	t.Setenv("KAFKA_GROUP_ID", "processing-v1")
	t.Setenv("KAFKA_FETCH_MIN_BYTES", " 1024 ")
	t.Setenv("KAFKA_FETCH_MAX_BYTES", " 2000000 ")
	t.Setenv("KAFKA_FETCH_MAX_WAIT", " 500ms ")
	t.Setenv("KAFKA_READ_BATCH_TIMEOUT", "3s")
	t.Setenv("KAFKA_QUEUE_CAPACITY", "25")
	t.Setenv("KAFKA_START_OFFSET", " latest ")
	cfg, err := LoadKafkaConsumer()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FetchMinBytes != 1024 || cfg.FetchMaxBytes != 2000000 || cfg.FetchMaxWait != 500*time.Millisecond || cfg.ReadBatchTimeout != 3*time.Second || cfg.QueueCapacity != 25 || cfg.StartOffset != KafkaStartLatest {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}

// TestLoadKafkaConsumerRejectsInvalidSettings protects startup from malformed fetch bounds and accidental offset fallback.
func TestLoadKafkaConsumerRejectsInvalidSettings(t *testing.T) {
	tests := []struct{ key, value string }{
		{"KAFKA_BROKERS", ""}, {"KAFKA_TOPIC", " "}, {"KAFKA_GROUP_ID", " "},
		{"KAFKA_START_OFFSET", ""}, {"KAFKA_START_OFFSET", "unknown"},
		{"KAFKA_FETCH_MIN_BYTES", "2000000"},
	}
	for _, key := range []string{"KAFKA_FETCH_MIN_BYTES", "KAFKA_FETCH_MAX_BYTES", "KAFKA_QUEUE_CAPACITY"} {
		for _, value := range []string{"", "invalid", "0", "-1", "999999999999999999999999"} {
			tests = append(tests, struct{ key, value string }{key, value})
		}
	}
	for _, key := range []string{"KAFKA_FETCH_MAX_WAIT", "KAFKA_READ_BATCH_TIMEOUT"} {
		for _, value := range []string{"", "invalid", "0s", "-1s"} {
			tests = append(tests, struct{ key, value string }{key, value})
		}
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			setKafkaConsumerFetchDefaults(t)
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_TOPIC", "events.v1")
			t.Setenv("KAFKA_GROUP_ID", "processing-v1")
			t.Setenv(tt.key, tt.value)
			if _, err := LoadKafkaConsumer(); err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("error = %v, want %s rejection", err, tt.key)
			}
		})
	}
}

func setKafkaConsumerFetchDefaults(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"KAFKA_FETCH_MIN_BYTES": "1", "KAFKA_FETCH_MAX_BYTES": "1000000", "KAFKA_FETCH_MAX_WAIT": "10s",
		"KAFKA_READ_BATCH_TIMEOUT": "10s", "KAFKA_QUEUE_CAPACITY": "100", "KAFKA_START_OFFSET": "earliest",
	} {
		t.Setenv(key, value)
	}
}
