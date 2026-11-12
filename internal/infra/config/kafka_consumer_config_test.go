package config

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestKafkaConsumerIsIndependent protects standalone consumers from invalid publisher policies and runtime values.
func TestKafkaConsumerIsIndependent(t *testing.T) {
	setKafkaConsumerFetchDefaults(t)
	t.Setenv("KAFKA_BROKERS", " localhost:9092 , localhost:19092 ")
	t.Setenv("KAFKA_GROUP_ID", " processing-v1 ")
	t.Setenv("KAFKA_PARTITION_STRATEGY", "unknown")
	t.Setenv("KAFKA_READ_TIMEOUT", "invalid")
	t.Setenv("KAFKA_WRITE_TIMEOUT", "invalid")
	t.Setenv("KAFKA_REQUIRED_ACKS", "invalid")
	t.Setenv("KAFKA_BATCH_SIZE", "invalid")
	t.Setenv("DB_URL", "")
	cfg, err := loadKafkaConsumer([]string{"events.v1"})
	if err != nil {
		t.Fatalf("loadKafkaConsumer error = %v", err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092", "localhost:19092"}) || !slices.Equal(cfg.Topics, []string{"events.v1"}) || cfg.GroupID != "processing-v1" {
		t.Fatalf("consumer settings = %+v, want normalized subscription settings", cfg)
	}
}

// TestKafkaConsumerDefaults protects a usable group subscription and explicit fetch bounds when settings are absent.
func TestKafkaConsumerDefaults(t *testing.T) {
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_GROUP_ID", "KAFKA_FETCH_MIN_BYTES", "KAFKA_FETCH_MAX_BYTES", "KAFKA_FETCH_MAX_WAIT", "KAFKA_REBALANCE_TIMEOUT", "KAFKA_MAX_CONCURRENT_FETCHES", "KAFKA_INITIAL_OFFSET", "KAFKA_REQUEST_TIMEOUT"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := loadKafkaConsumer([]string{"notifications.requested.v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092"}) || !slices.Equal(cfg.Topics, []string{"notifications.requested.v1"}) || cfg.GroupID != "notification-sender-v1" || cfg.FetchMinBytes != 1 || cfg.FetchMaxBytes != 1000000 || cfg.FetchMaxWait != 10*time.Second || cfg.RequestTimeout != 10*time.Second || cfg.RebalanceTimeout != time.Minute || cfg.MaxConcurrentFetches != 1 || cfg.InitialOffset != KafkaInitialEarliest {
		t.Fatalf("unexpected consumer defaults: %+v", cfg)
	}
}

// TestKafkaConsumerFetchOverrides protects normalized tuning and latest startup without depending on publisher settings.
func TestKafkaConsumerFetchOverrides(t *testing.T) {
	setKafkaConsumerFetchDefaults(t)
	t.Setenv("KAFKA_BROKERS", "localhost:9092")
	t.Setenv("KAFKA_GROUP_ID", "processing-v1")
	t.Setenv("KAFKA_FETCH_MIN_BYTES", " 1024 ")
	t.Setenv("KAFKA_FETCH_MAX_BYTES", " 2000000 ")
	t.Setenv("KAFKA_FETCH_MAX_WAIT", " 500ms ")
	t.Setenv("KAFKA_REBALANCE_TIMEOUT", "3s")
	t.Setenv("KAFKA_MAX_CONCURRENT_FETCHES", "25")
	t.Setenv("KAFKA_INITIAL_OFFSET", " latest ")
	cfg, err := loadKafkaConsumer([]string{"events.v1"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FetchMinBytes != 1024 || cfg.FetchMaxBytes != 2000000 || cfg.FetchMaxWait != 500*time.Millisecond || cfg.RebalanceTimeout != 3*time.Second || cfg.MaxConcurrentFetches != 25 || cfg.InitialOffset != KafkaInitialLatest {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}

// TestKafkaConsumerRejectsInvalidSettings protects startup from malformed fetch bounds and accidental offset fallback.
func TestKafkaConsumerRejectsInvalidSettings(t *testing.T) {
	tests := []struct{ key, value string }{
		{"KAFKA_BROKERS", ""}, {"KAFKA_GROUP_ID", " "},
		{"KAFKA_INITIAL_OFFSET", ""}, {"KAFKA_INITIAL_OFFSET", "unknown"},
		{"KAFKA_FETCH_MIN_BYTES", "2000000"},
	}
	for _, key := range []string{"KAFKA_FETCH_MIN_BYTES", "KAFKA_FETCH_MAX_BYTES", "KAFKA_MAX_CONCURRENT_FETCHES"} {
		for _, value := range []string{"", "invalid", "0", "-1", "999999999999999999999999"} {
			tests = append(tests, struct{ key, value string }{key, value})
		}
	}
	for _, key := range []string{"KAFKA_FETCH_MAX_WAIT", "KAFKA_REBALANCE_TIMEOUT"} {
		for _, value := range []string{"", "invalid", "0s", "-1s"} {
			tests = append(tests, struct{ key, value string }{key, value})
		}
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			setKafkaConsumerFetchDefaults(t)
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_GROUP_ID", "processing-v1")
			t.Setenv(tt.key, tt.value)
			if _, err := loadKafkaConsumer([]string{"events.v1"}); err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("error = %v, want %s rejection", err, tt.key)
			}
		})
	}
}

func setKafkaConsumerFetchDefaults(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"KAFKA_FETCH_MIN_BYTES": "1", "KAFKA_FETCH_MAX_BYTES": "1000000", "KAFKA_FETCH_MAX_WAIT": "10s",
		"KAFKA_REBALANCE_TIMEOUT": "1m", "KAFKA_REQUEST_TIMEOUT": "10s", "KAFKA_MAX_CONCURRENT_FETCHES": "1", "KAFKA_INITIAL_OFFSET": "earliest",
	} {
		t.Setenv(key, value)
	}
}

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
	t.Setenv("KAFKA_NOTIFICATION_TOPIC", " notifications.requested.v1 ")
	t.Setenv("KAFKA_GROUP_ID", " notification-sender-v1 ")
	t.Setenv("KAFKA_PARTITION_STRATEGY", " key ")
	t.Setenv("KAFKA_READ_TIMEOUT", " 2s ")
	t.Setenv("KAFKA_WRITE_TIMEOUT", "3s")

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
	if !slices.Equal(cfg.Kafka.Topics, []string{"notifications.requested.v1", "activities.recorded.v1"}) {
		t.Fatalf("Kafka topic = %q, want notification and activity topics", cfg.Kafka.Topics)
	}
	if cfg.Kafka.GroupID != "notification-sender-v1" {
		t.Fatalf("Kafka group ID = %q, want notification-sender-v1", cfg.Kafka.GroupID)
	}
}

// TestLoadKafkaWorkerUsesLocalDefaults protects a runnable local setup when Kafka environment variables are unset.
func TestLoadKafkaWorkerUsesLocalDefaults(t *testing.T) {
	setValidRuntimeEnv(t)
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_NOTIFICATION_TOPIC", "KAFKA_GROUP_ID", "KAFKA_PARTITION_STRATEGY", "KAFKA_READ_TIMEOUT", "KAFKA_WRITE_TIMEOUT"} {
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
	if !slices.Equal(cfg.Kafka.Topics, []string{"notifications.requested.v1", "activities.recorded.v1"}) {
		t.Fatalf("Kafka topic = %q, want notification and activity topics", cfg.Kafka.Topics)
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
		{name: "empty topic list", key: "KAFKA_NOTIFICATION_TOPIC", value: ""},
		{name: "blank topic list", key: "KAFKA_NOTIFICATION_TOPIC", value: "   "},
		{name: "empty group ID", key: "KAFKA_GROUP_ID", value: ""},
		{name: "blank group ID", key: "KAFKA_GROUP_ID", value: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidRuntimeEnv(t)
			t.Setenv("KAFKA_PARTITION_STRATEGY", "round-robin")
			t.Setenv("KAFKA_READ_TIMEOUT", "10s")
			t.Setenv("KAFKA_WRITE_TIMEOUT", "10s")
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_NOTIFICATION_TOPIC", "notifications.requested.v1")
			t.Setenv("KAFKA_GROUP_ID", "notification-sender-v1")
			t.Setenv("KAFKA_ACTIVITY_TOPIC", "")
			t.Setenv(tt.key, tt.value)

			_, err := LoadKafkaWorker()
			if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("LoadKafkaWorker() error = %v, want an error identifying %s", err, tt.key)
			}
		})
	}
}

// TestLoadKafkaWorkerIgnoresPublisherPolicies protects consumer-only startup while preserving the shared runtime configuration.
func TestLoadKafkaWorkerIgnoresPublisherPolicies(t *testing.T) {
	setValidRuntimeEnv(t)
	t.Setenv("KAFKA_BROKERS", "localhost:9092")
	t.Setenv("KAFKA_NOTIFICATION_TOPIC", "events.v1")
	t.Setenv("KAFKA_GROUP_ID", "processing-v1")
	t.Setenv("KAFKA_PARTITION_STRATEGY", "unknown")
	t.Setenv("KAFKA_READ_TIMEOUT", "invalid")
	t.Setenv("KAFKA_WRITE_TIMEOUT", "invalid")
	if _, err := LoadKafkaWorker(); err != nil {
		t.Fatalf("LoadKafkaWorker() error = %v", err)
	}
}

// TestKafkaWorkerHandlingTimeout validates processing budgets at startup while
// keeping them independent of the reader's network timeouts.
func TestKafkaWorkerHandlingTimeout(t *testing.T) {
	for _, value := range []string{"2s", "0", "-1s", "bad"} {
		t.Run(value, func(t *testing.T) {
			setValidRuntimeEnv(t)
			t.Setenv("KAFKA_HANDLING_TIMEOUT", value)
			cfg, err := LoadKafkaWorker()
			if value == "2s" {
				if err != nil || cfg.HandlingTimeout.String() != value {
					t.Fatalf("timeout=%v error=%v", cfg.HandlingTimeout, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "KAFKA_HANDLING_TIMEOUT") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
