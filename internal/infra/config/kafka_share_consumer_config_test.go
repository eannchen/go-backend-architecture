package config

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestKafkaShareConsumerDefaults protects an independent, bounded share
// subscription without borrowing ordinary offset/rebalance configuration.
func TestKafkaShareConsumerDefaults(t *testing.T) {
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_NOTIFICATION_SHARE_GROUP_ID", "KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS", "KAFKA_NOTIFICATION_SHARE_FETCH_MIN_BYTES", "KAFKA_NOTIFICATION_SHARE_FETCH_MAX_BYTES", "KAFKA_NOTIFICATION_SHARE_FETCH_MAX_WAIT", "KAFKA_NOTIFICATION_SHARE_REQUEST_TIMEOUT"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("KAFKA_NOTIFICATION_GROUP_ID", "")
	t.Setenv("KAFKA_NOTIFICATION_INITIAL_OFFSET", "invalid")
	t.Setenv("KAFKA_NOTIFICATION_REBALANCE_TIMEOUT", "invalid")
	t.Setenv("KAFKA_REQUIRED_ACKS", "invalid")
	t.Setenv("KAFKA_NOTIFICATION_REQUEST_TIMEOUT", "invalid")
	t.Setenv("DB_URL", "")
	cfg, err := loadKafkaShareConsumer([]string{"notifications.requested.v1"}, "KAFKA_NOTIFICATION_", "shared-workers-v1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092"}) || !slices.Equal(cfg.Topics, []string{"notifications.requested.v1"}) || cfg.GroupID != "shared-workers-v1" || cfg.MaxPollRecords != 100 || cfg.FetchMinBytes != 1 || cfg.FetchMaxBytes != 1000000 || cfg.FetchMaxWait != 500*time.Millisecond || cfg.RequestTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

// TestKafkaShareConsumerOverrides verifies caller-selected buffering and
// normalized addresses without exposing SDK option types in configuration.
func TestKafkaShareConsumerOverrides(t *testing.T) {
	setShareConsumerEnv(t)
	t.Setenv("KAFKA_BROKERS", " localhost:9092 , localhost:19092 ")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_GROUP_ID", " shared-jobs ")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS", " 5 ")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_FETCH_MIN_BYTES", "1024")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_FETCH_MAX_BYTES", "2000000")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_FETCH_MAX_WAIT", "1s")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_REQUEST_TIMEOUT", "2s")
	cfg, err := loadKafkaShareConsumer([]string{"jobs.v1"}, "KAFKA_NOTIFICATION_", "shared-workers-v1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092", "localhost:19092"}) || !slices.Equal(cfg.Topics, []string{"jobs.v1"}) || cfg.GroupID != "shared-jobs" || cfg.MaxPollRecords != 5 || cfg.FetchMinBytes != 1024 || cfg.FetchMaxBytes != 2000000 || cfg.FetchMaxWait != time.Second || cfg.RequestTimeout != 2*time.Second {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}

// TestKafkaShareConsumerValidation rejects malformed values and unsafe
// bounds at startup rather than silently falling back to library defaults.
func TestKafkaShareConsumerValidation(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"KAFKA_BROKERS", ""}, {"KAFKA_NOTIFICATION_SHARE_GROUP_ID", " "},
		{"KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS", "invalid"}, {"KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS", "0"}, {"KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS", "10001"},
		{"KAFKA_NOTIFICATION_SHARE_FETCH_MIN_BYTES", "0"}, {"KAFKA_NOTIFICATION_SHARE_FETCH_MIN_BYTES", "1000001"},
		{"KAFKA_NOTIFICATION_SHARE_FETCH_MAX_BYTES", "invalid"}, {"KAFKA_NOTIFICATION_SHARE_FETCH_MAX_BYTES", "2147483648"},
		{"KAFKA_NOTIFICATION_SHARE_FETCH_MAX_WAIT", "invalid"}, {"KAFKA_NOTIFICATION_SHARE_FETCH_MAX_WAIT", "1ms"}, {"KAFKA_NOTIFICATION_SHARE_FETCH_MAX_WAIT", "1000h"},
		{"KAFKA_NOTIFICATION_SHARE_REQUEST_TIMEOUT", "invalid"}, {"KAFKA_NOTIFICATION_SHARE_REQUEST_TIMEOUT", "1ms"}, {"KAFKA_NOTIFICATION_SHARE_REQUEST_TIMEOUT", "16m"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			setShareConsumerEnv(t)
			t.Setenv(tc.key, tc.value)
			if _, err := loadKafkaShareConsumer([]string{"jobs.v1"}, "KAFKA_NOTIFICATION_", "shared-workers-v1"); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error=%v want %s", err, tc.key)
			}
		})
	}
}

func setShareConsumerEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"KAFKA_BROKERS": "localhost:9092", "KAFKA_NOTIFICATION_SHARE_GROUP_ID": "shared-jobs",
		"KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS": "100", "KAFKA_NOTIFICATION_SHARE_FETCH_MIN_BYTES": "1", "KAFKA_NOTIFICATION_SHARE_FETCH_MAX_BYTES": "1000000",
		"KAFKA_NOTIFICATION_SHARE_FETCH_MAX_WAIT": "500ms", "KAFKA_NOTIFICATION_SHARE_REQUEST_TIMEOUT": "10s",
	} {
		t.Setenv(key, value)
	}
}

// TestShareWorkerConfigKeepsRuntimeAndBoundsWork verifies shared settings remain
// available while worker defaults avoid acquiring a long queue of locked records.
func TestShareWorkerConfigKeepsRuntimeAndBoundsWork(t *testing.T) {
	setValidRuntimeEnv(t)
	for _, key := range []string{"KAFKA_NOTIFICATION_SHARE_WORKER_CONCURRENCY", "KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS", "KAFKA_NOTIFICATION_SHARE_HANDLING_TIMEOUT", "KAFKA_NOTIFICATION_SHARE_ACKNOWLEDGMENT_MARGIN"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("KAFKA_NOTIFICATION_SHARE_GROUP_ID", "share-workers")
	t.Setenv("KAFKA_BROKERS", "localhost:9092")
	t.Setenv("KAFKA_NOTIFICATION_TOPIC", "notifications.requested.v1")
	t.Setenv("KAFKA_NOTIFICATION_GROUP_ID", "")     // Ordinary consumer settings do not apply.
	t.Setenv("KAFKA_PARTITION_STRATEGY", "invalid") // Publisher settings do not apply.
	runtime, err := LoadRuntime()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadKafkaShareWorker()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RuntimeConfig != runtime || cfg.Readers[0].Concurrency != 4 || cfg.Readers[0].Kafka.MaxPollRecords != 4 || cfg.Readers[0].HandlingTimeout != 10*time.Second || cfg.Readers[0].AcknowledgmentMargin != 3*time.Second {
		t.Fatalf("config=%+v", cfg)
	}
	t.Setenv("KAFKA_NOTIFICATION_SHARE_WORKER_CONCURRENCY", " 2 ")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_MAX_POLL_RECORDS", "6")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_HANDLING_TIMEOUT", "5s")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_ACKNOWLEDGMENT_MARGIN", "2s")
	cfg, err = LoadKafkaShareWorker()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Readers[0].Concurrency != 2 || cfg.Readers[0].Kafka.MaxPollRecords != 6 || cfg.Readers[0].HandlingTimeout != 5*time.Second || cfg.Readers[0].AcknowledgmentMargin != 2*time.Second {
		t.Fatalf("overrides=%+v", cfg)
	}
}

// TestShareWorkerConfigRejectsInvalidSettings prevents malformed values from
// silently creating an unbounded pool or removing the handler deadline.
func TestShareWorkerConfigRejectsInvalidSettings(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"KAFKA_NOTIFICATION_SHARE_WORKER_CONCURRENCY", "0"}, {"KAFKA_NOTIFICATION_SHARE_WORKER_CONCURRENCY", "1001"}, {"KAFKA_NOTIFICATION_SHARE_WORKER_CONCURRENCY", "many"},
		{"KAFKA_NOTIFICATION_SHARE_HANDLING_TIMEOUT", "0s"}, {"KAFKA_NOTIFICATION_SHARE_HANDLING_TIMEOUT", "bad"}, {"KAFKA_NOTIFICATION_SHARE_ACKNOWLEDGMENT_MARGIN", "-1s"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			setValidRuntimeEnv(t)
			t.Setenv("KAFKA_NOTIFICATION_SHARE_WORKER_CONCURRENCY", "4")
			t.Setenv("KAFKA_NOTIFICATION_SHARE_HANDLING_TIMEOUT", "10s")
			t.Setenv("KAFKA_NOTIFICATION_SHARE_ACKNOWLEDGMENT_MARGIN", "3s")
			t.Setenv(tc.key, tc.value)
			if _, err := LoadKafkaShareWorker(); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error=%v want %s", err, tc.key)
			}
		})
	}
}

// TestWorkflowShareSettingsAreIndependent protects per-workflow pool sizing,
// automatic acquisition limits, and explicit batch overrides.
func TestWorkflowShareSettingsAreIndependent(t *testing.T) {
	setValidRuntimeEnv(t)
	t.Setenv("KAFKA_NOTIFICATION_TOPIC", "mail")
	t.Setenv("KAFKA_ACTIVITY_TOPIC", "actions")
	for name, concurrency := range map[string]string{"NOTIFICATION": "2", "ACTIVITY": "5"} {
		prefix := "KAFKA_" + name + "_SHARE_"
		t.Setenv(prefix+"GROUP_ID", strings.ToLower(name)+"-share")
		t.Setenv(prefix+"WORKER_CONCURRENCY", concurrency)
		t.Setenv(prefix+"MAX_POLL_RECORDS", "")
		if err := os.Unsetenv(prefix + "MAX_POLL_RECORDS"); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := LoadKafkaShareWorker()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Readers) != 2 {
		t.Fatalf("readers = %d", len(cfg.Readers))
	}
	for i, want := range []int{2, 5} {
		if cfg.Readers[i].Concurrency != want || cfg.Readers[i].Kafka.MaxPollRecords != want {
			t.Fatalf("reader %d = %+v", i, cfg.Readers[i])
		}
	}
	t.Setenv("KAFKA_ACTIVITY_SHARE_MAX_POLL_RECORDS", "9")
	t.Setenv("KAFKA_NOTIFICATION_SHARE_HANDLING_TIMEOUT", "1s")
	t.Setenv("KAFKA_ACTIVITY_SHARE_HANDLING_TIMEOUT", "3s")
	cfg, err = LoadKafkaShareWorker()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Readers[0].Kafka.MaxPollRecords != 2 || cfg.Readers[1].Kafka.MaxPollRecords != 9 ||
		cfg.Readers[0].HandlingTimeout != time.Second || cfg.Readers[1].HandlingTimeout != 3*time.Second {
		t.Fatalf("settings = %+v", cfg.Readers)
	}
	t.Setenv("KAFKA_ACTIVITY_TOPIC", "")
	t.Setenv("KAFKA_ACTIVITY_SHARE_WORKER_CONCURRENCY", "invalid")
	cfg, err = LoadKafkaShareWorker()
	if err != nil || len(cfg.Readers) != 1 || cfg.Readers[0].Concurrency != 2 {
		t.Fatalf("disabled workflow: %+v / %v", cfg, err)
	}
}
