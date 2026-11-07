package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestShareWorkerConfigKeepsRuntimeAndBoundsWork verifies shared settings remain
// available while worker defaults avoid acquiring a long queue of locked records.
func TestShareWorkerConfigKeepsRuntimeAndBoundsWork(t *testing.T) {
	setValidRuntimeEnv(t)
	for _, key := range []string{"KAFKA_SHARE_WORKER_CONCURRENCY", "KAFKA_SHARE_MAX_POLL_RECORDS", "KAFKA_SHARE_HANDLING_TIMEOUT", "KAFKA_SHARE_ACKNOWLEDGMENT_MARGIN"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("KAFKA_SHARE_GROUP_ID", "share-workers")
	t.Setenv("KAFKA_BROKERS", "localhost:9092")
	t.Setenv("KAFKA_TOPIC", "notifications.requested.v1")
	t.Setenv("KAFKA_GROUP_ID", "")                  // Ordinary consumer settings do not apply.
	t.Setenv("KAFKA_PARTITION_STRATEGY", "invalid") // Publisher settings do not apply.
	runtime, err := LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadKafkaShareWorker()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RuntimeConfig != runtime || cfg.Concurrency != 4 || cfg.Kafka.MaxPollRecords != 4 || cfg.HandlingTimeout != 10*time.Second || cfg.AcknowledgmentMargin != 3*time.Second {
		t.Fatalf("config=%+v", cfg)
	}
	t.Setenv("KAFKA_SHARE_WORKER_CONCURRENCY", " 2 ")
	t.Setenv("KAFKA_SHARE_MAX_POLL_RECORDS", "6")
	t.Setenv("KAFKA_SHARE_HANDLING_TIMEOUT", "5s")
	t.Setenv("KAFKA_SHARE_ACKNOWLEDGMENT_MARGIN", "2s")
	cfg, err = LoadKafkaShareWorker()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Concurrency != 2 || cfg.Kafka.MaxPollRecords != 6 || cfg.HandlingTimeout != 5*time.Second || cfg.AcknowledgmentMargin != 2*time.Second {
		t.Fatalf("overrides=%+v", cfg)
	}
}

// TestShareWorkerConfigRejectsInvalidSettings prevents malformed values from
// silently creating an unbounded pool or removing the handler deadline.
func TestShareWorkerConfigRejectsInvalidSettings(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"KAFKA_SHARE_WORKER_CONCURRENCY", "0"}, {"KAFKA_SHARE_WORKER_CONCURRENCY", "1001"}, {"KAFKA_SHARE_WORKER_CONCURRENCY", "many"},
		{"KAFKA_SHARE_HANDLING_TIMEOUT", "0s"}, {"KAFKA_SHARE_HANDLING_TIMEOUT", "bad"}, {"KAFKA_SHARE_ACKNOWLEDGMENT_MARGIN", "-1s"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			setValidRuntimeEnv(t)
			t.Setenv("KAFKA_SHARE_WORKER_CONCURRENCY", "4")
			t.Setenv("KAFKA_SHARE_HANDLING_TIMEOUT", "10s")
			t.Setenv("KAFKA_SHARE_ACKNOWLEDGMENT_MARGIN", "3s")
			t.Setenv(tc.key, tc.value)
			if _, err := LoadKafkaShareWorker(); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error=%v want %s", err, tc.key)
			}
		})
	}
}
