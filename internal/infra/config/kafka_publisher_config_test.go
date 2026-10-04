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
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_PARTITION_STRATEGY", "KAFKA_READ_TIMEOUT", "KAFKA_WRITE_TIMEOUT"} {
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
}

// TestLoadKafkaPublisherRejectsMissingConnection protects publishers from empty cluster addresses and topics without requiring a consumer group.
func TestLoadKafkaPublisherRejectsMissingConnection(t *testing.T) {
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
