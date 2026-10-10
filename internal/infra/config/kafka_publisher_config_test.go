package config

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestLoadKafkaPublisherPolicies protects selectable routing and rejects malformed deadlines at startup.
func TestLoadKafkaPublisherPolicies(t *testing.T) {
	setKafkaPublisherTuningDefaults(t)
	tests := []struct {
		key, value string
		valid      bool
	}{
		{"KAFKA_PARTITION_STRATEGY", "round-robin", true},
		{"KAFKA_PARTITION_STRATEGY", "key", true},
		{"KAFKA_PARTITION_STRATEGY", "", false},
		{"KAFKA_PARTITION_STRATEGY", "unknown", false},
		{"KAFKA_READ_TIMEOUT", "0s", false},
		{"KAFKA_READ_TIMEOUT", "-1s", false},
		{"KAFKA_READ_TIMEOUT", "ten seconds", false},
		{"KAFKA_READ_TIMEOUT", "", false},
		{"KAFKA_WRITE_TIMEOUT", "0s", false},
		{"KAFKA_WRITE_TIMEOUT", "-1s", false},
		{"KAFKA_WRITE_TIMEOUT", "invalid", false},
		{"KAFKA_WRITE_TIMEOUT", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_TOPIC", "notifications.requested.v1")
			t.Setenv("KAFKA_PARTITION_STRATEGY", "round-robin")
			t.Setenv("KAFKA_READ_TIMEOUT", "10s")
			t.Setenv("KAFKA_WRITE_TIMEOUT", "10s")
			t.Setenv(tt.key, tt.value)
			cfg, err := LoadKafkaPublisher()
			if tt.valid {
				if err != nil {
					t.Fatalf("LoadKafkaPublisher() error = %v", err)
				}
				if string(cfg.PartitionStrategy) != tt.value {
					t.Fatalf("strategy = %q, want %q", cfg.PartitionStrategy, tt.value)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("LoadKafkaPublisher() error = %v, want error identifying %s", err, tt.key)
			}
		})
	}
}

// TestLoadKafkaPublisherIsIndependent protects publisher-only processes from unrelated consumer and runtime settings.
func TestLoadKafkaPublisherIsIndependent(t *testing.T) {
	setKafkaPublisherTuningDefaults(t)
	t.Setenv("KAFKA_BROKERS", " localhost:9092 , localhost:19092 ")
	t.Setenv("KAFKA_TOPIC", " events.v1 ")
	t.Setenv("KAFKA_PARTITION_STRATEGY", " key ")
	t.Setenv("KAFKA_READ_TIMEOUT", " 2s ")
	t.Setenv("KAFKA_WRITE_TIMEOUT", "3s")
	t.Setenv("KAFKA_GROUP_ID", "")
	t.Setenv("DB_URL", "")
	cfg, err := LoadKafkaPublisher()
	if err != nil {
		t.Fatalf("LoadKafkaPublisher() error = %v", err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092", "localhost:19092"}) || cfg.Topic != "events.v1" || cfg.PartitionStrategy != PartitionByKey || cfg.ReadTimeout != 2*time.Second || cfg.WriteTimeout != 3*time.Second {
		t.Fatalf("publisher settings = %+v, want normalized publisher-only settings", cfg)
	}
}

// TestLoadKafkaPublisherDefaults protects standalone publication with documented local defaults.
func TestLoadKafkaPublisherDefaults(t *testing.T) {
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_PARTITION_STRATEGY", "KAFKA_READ_TIMEOUT", "KAFKA_WRITE_TIMEOUT", "KAFKA_BATCH_SIZE", "KAFKA_BATCH_BYTES", "KAFKA_BATCH_TIMEOUT", "KAFKA_MAX_ATTEMPTS", "KAFKA_WRITE_BACKOFF_MIN", "KAFKA_WRITE_BACKOFF_MAX", "KAFKA_COMPRESSION", "KAFKA_REQUIRED_ACKS"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := LoadKafkaPublisher()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092"}) || cfg.Topic != "notifications.requested.v1" || cfg.PartitionStrategy != PartitionRoundRobin || cfg.ReadTimeout != 10*time.Second || cfg.WriteTimeout != 10*time.Second {
		t.Fatalf("unexpected publisher defaults: %+v", cfg)
	}
	if cfg.RequiredAcks != KafkaAcksAll || cfg.BatchSize != 100 || cfg.BatchBytes != 1048576 || cfg.BatchTimeout != time.Second || cfg.MaxAttempts != 10 || cfg.WriteBackoffMin != 100*time.Millisecond || cfg.WriteBackoffMax != time.Second || cfg.Compression != CompressionNone {
		t.Fatalf("unexpected tuning defaults: %+v", cfg)
	}

}

// TestLoadKafkaPublisherRejectsMissingConnection protects publishers from empty cluster addresses and topics without requiring a consumer group.
func TestLoadKafkaPublisherRejectsMissingConnection(t *testing.T) {
	setKafkaPublisherTuningDefaults(t)
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_TOPIC", "events.v1")
			t.Setenv("KAFKA_PARTITION_STRATEGY", "round-robin")
			t.Setenv("KAFKA_READ_TIMEOUT", "10s")
			t.Setenv("KAFKA_WRITE_TIMEOUT", "10s")
			t.Setenv(key, " ")
			if _, err := LoadKafkaPublisher(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("error = %v, want %s validation", err, key)
			}
		})
	}
}

// TestLoadKafkaPublisherTuning protects explicit overrides and strict parsing instead of silent SDK fallback.
func TestLoadKafkaPublisherTuning(t *testing.T) {
	tests := []struct {
		key, value string
		valid      bool
	}{
		{"KAFKA_BATCH_SIZE", " 1 ", true}, {"KAFKA_BATCH_BYTES", " 65536 ", true},
		{"KAFKA_BATCH_TIMEOUT", "50ms", true}, {"KAFKA_MAX_ATTEMPTS", "1", true},
		{"KAFKA_WRITE_BACKOFF_MIN", "200ms", true}, {"KAFKA_WRITE_BACKOFF_MAX", "2s", true},
		{"KAFKA_COMPRESSION", "none", true}, {"KAFKA_COMPRESSION", " gzip ", true},
		{"KAFKA_COMPRESSION", "snappy", true}, {"KAFKA_COMPRESSION", "lz4", true}, {"KAFKA_COMPRESSION", "zstd", true},
	}
	for _, key := range []string{"KAFKA_BATCH_SIZE", "KAFKA_BATCH_BYTES", "KAFKA_BATCH_TIMEOUT", "KAFKA_MAX_ATTEMPTS", "KAFKA_WRITE_BACKOFF_MIN", "KAFKA_WRITE_BACKOFF_MAX"} {
		for _, value := range []string{"", "invalid", "0", "-1"} {
			tests = append(tests, struct {
				key, value string
				valid      bool
			}{key, value, false})
		}
	}
	tests = append(tests, struct {
		key, value string
		valid      bool
	}{"KAFKA_COMPRESSION", "unknown", false}, struct {
		key, value string
		valid      bool
	}{"KAFKA_COMPRESSION", "", false}, struct {
		key, value string
		valid      bool
	}{"KAFKA_WRITE_BACKOFF_MIN", "2s", false}, struct {
		key, value string
		valid      bool
	}{"KAFKA_BATCH_BYTES", "9223372036854775808", false})
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			setKafkaPublisherTuningDefaults(t)
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_TOPIC", "events.v1")
			t.Setenv("KAFKA_PARTITION_STRATEGY", "round-robin")
			t.Setenv("KAFKA_READ_TIMEOUT", "10s")
			t.Setenv("KAFKA_WRITE_TIMEOUT", "10s")
			t.Setenv(tt.key, tt.value)
			cfg, err := LoadKafkaPublisher()
			if !tt.valid {
				if err == nil || !strings.Contains(err.Error(), tt.key) {
					t.Fatalf("error = %v, want %s rejection", err, tt.key)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch tt.key {
			case "KAFKA_BATCH_SIZE":
				if cfg.BatchSize != 1 {
					t.Fatalf("batch size = %d", cfg.BatchSize)
				}
			case "KAFKA_BATCH_BYTES":
				if cfg.BatchBytes != 65536 {
					t.Fatalf("batch bytes = %d", cfg.BatchBytes)
				}
			case "KAFKA_BATCH_TIMEOUT":
				if cfg.BatchTimeout != 50*time.Millisecond {
					t.Fatalf("batch timeout = %v", cfg.BatchTimeout)
				}
			case "KAFKA_MAX_ATTEMPTS":
				if cfg.MaxAttempts != 1 {
					t.Fatalf("max attempts = %d", cfg.MaxAttempts)
				}
			case "KAFKA_WRITE_BACKOFF_MIN":
				if cfg.WriteBackoffMin != 200*time.Millisecond {
					t.Fatalf("min backoff = %v", cfg.WriteBackoffMin)
				}
			case "KAFKA_WRITE_BACKOFF_MAX":
				if cfg.WriteBackoffMax != 2*time.Second {
					t.Fatalf("max backoff = %v", cfg.WriteBackoffMax)
				}
			case "KAFKA_COMPRESSION":
				if string(cfg.Compression) != strings.TrimSpace(tt.value) {
					t.Fatalf("compression = %q", cfg.Compression)
				}
			}
		})
	}
}

func setKafkaPublisherTuningDefaults(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"KAFKA_BATCH_SIZE": "100", "KAFKA_BATCH_BYTES": "1048576", "KAFKA_BATCH_TIMEOUT": "1s",
		"KAFKA_MAX_ATTEMPTS": "10", "KAFKA_WRITE_BACKOFF_MIN": "100ms", "KAFKA_WRITE_BACKOFF_MAX": "1s", "KAFKA_COMPRESSION": "none", "KAFKA_REQUIRED_ACKS": "all",
	} {
		t.Setenv(key, value)
	}
}

// TestLoadKafkaPublisherAcknowledgments protects selectable confirmation and rejects typos rather than silently weakening durability.
func TestLoadKafkaPublisherAcknowledgments(t *testing.T) {
	tests := []struct {
		value string
		want  KafkaAcknowledgments
		valid bool
	}{
		{"all", KafkaAcksAll, true}, {" leader ", KafkaAcksLeader, true}, {"none", KafkaAcksNone, true},
		{"", "", false}, {"   ", "", false}, {"unknown", "", false}, {"1", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			setKafkaPublisherTuningDefaults(t)
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_TOPIC", "events.v1")
			t.Setenv("KAFKA_PARTITION_STRATEGY", "round-robin")
			t.Setenv("KAFKA_READ_TIMEOUT", "10s")
			t.Setenv("KAFKA_WRITE_TIMEOUT", "10s")
			t.Setenv("KAFKA_REQUIRED_ACKS", tt.value)
			cfg, err := LoadKafkaPublisher()
			if !tt.valid {
				if err == nil || !strings.Contains(err.Error(), "KAFKA_REQUIRED_ACKS") {
					t.Fatalf("error = %v, want acknowledgment validation", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.RequiredAcks != tt.want {
				t.Fatalf("RequiredAcks = %q, want %q", cfg.RequiredAcks, tt.want)
			}
		})
	}
}
