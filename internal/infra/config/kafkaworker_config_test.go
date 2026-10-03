package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestLoadKafkaWorkerReadsSettingsAndReusesRuntime protects the worker's Kafka subscription while retaining shared settings for cloned projects.
func TestLoadKafkaWorkerReadsSettingsAndReusesRuntime(t *testing.T) {
	setValidRuntimeEnv(t)
	t.Setenv("SERVICE_NAME", " notification-worker ")
	t.Setenv("DB_URL", " postgres://postgres:postgres@localhost:15432/app?sslmode=disable ")
	t.Setenv("REDIS_ADDR", " localhost:16379 ")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("OTEL_EXPORT_ENABLED", "false")
	t.Setenv("SHUTDOWN_GRACE_PERIOD", "3s")
	t.Setenv("KAFKA_BROKERS", " localhost:9092 , localhost:19092 ")
	t.Setenv("KAFKA_TOPIC", " notifications.requested.v1 ")
	t.Setenv("KAFKA_GROUP_ID", " notification-sender-v1 ")

	wantRuntime, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}

	cfg, err := LoadKafkaWorker()
	if err != nil {
		t.Fatalf("LoadKafkaWorker() error = %v", err)
	}
	if cfg.RuntimeConfig != wantRuntime {
		t.Fatal("worker runtime settings differ from LoadRuntime(), want the complete shared configuration")
	}
	wantBrokers := []string{"localhost:9092", "localhost:19092"}
	if !slices.Equal(cfg.Kafka.Brokers, wantBrokers) {
		t.Fatalf("Kafka brokers = %v, want %v", cfg.Kafka.Brokers, wantBrokers)
	}
	if cfg.Kafka.Topic != "notifications.requested.v1" {
		t.Fatalf("Kafka topic = %q, want notifications.requested.v1", cfg.Kafka.Topic)
	}
	if cfg.Kafka.GroupID != "notification-sender-v1" {
		t.Fatalf("Kafka group ID = %q, want notification-sender-v1", cfg.Kafka.GroupID)
	}
}

// TestLoadKafkaWorkerUsesLocalDefaults protects a runnable local setup when Kafka environment variables are unset.
func TestLoadKafkaWorkerUsesLocalDefaults(t *testing.T) {
	setValidRuntimeEnv(t)
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP_ID"} {
		// Register restoration before unsetting so inherited environment values cannot affect the defaults.
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}

	cfg, err := LoadKafkaWorker()
	if err != nil {
		t.Fatalf("LoadKafkaWorker() error = %v", err)
	}
	if !slices.Equal(cfg.Kafka.Brokers, []string{"localhost:9092"}) {
		t.Fatalf("Kafka brokers = %v, want [localhost:9092]", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "notifications.requested.v1" {
		t.Fatalf("Kafka topic = %q, want notifications.requested.v1", cfg.Kafka.Topic)
	}
	if cfg.Kafka.GroupID != "notification-sender-v1" {
		t.Fatalf("Kafka group ID = %q, want notification-sender-v1", cfg.Kafka.GroupID)
	}
}

// TestLoadKafkaWorkerRejectsEmptySubscriptionSettings protects startup from explicit empty values instead of silently using defaults.
func TestLoadKafkaWorkerRejectsEmptySubscriptionSettings(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "empty brokers", key: "KAFKA_BROKERS", value: ""},
		{name: "blank brokers", key: "KAFKA_BROKERS", value: "   "},
		{name: "brokers with only separators", key: "KAFKA_BROKERS", value: " , , "},
		{name: "empty topic", key: "KAFKA_TOPIC", value: ""},
		{name: "blank topic", key: "KAFKA_TOPIC", value: "   "},
		{name: "empty group ID", key: "KAFKA_GROUP_ID", value: ""},
		{name: "blank group ID", key: "KAFKA_GROUP_ID", value: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidRuntimeEnv(t)
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_TOPIC", "notifications.requested.v1")
			t.Setenv("KAFKA_GROUP_ID", "notification-sender-v1")
			t.Setenv(tt.key, tt.value)

			_, err := LoadKafkaWorker()
			if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("LoadKafkaWorker() error = %v, want an error identifying %s", err, tt.key)
			}
		})
	}
}
