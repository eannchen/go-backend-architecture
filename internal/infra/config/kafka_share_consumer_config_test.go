package config

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestLoadKafkaShareConsumerDefaults protects an independent, bounded share
// subscription without borrowing ordinary offset/rebalance configuration.
func TestLoadKafkaShareConsumerDefaults(t *testing.T) {
	for _, key := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_SHARE_GROUP_ID", "KAFKA_SHARE_MAX_POLL_RECORDS", "KAFKA_SHARE_FETCH_MIN_BYTES", "KAFKA_SHARE_FETCH_MAX_BYTES", "KAFKA_SHARE_FETCH_MAX_WAIT", "KAFKA_SHARE_REQUEST_TIMEOUT"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("KAFKA_GROUP_ID", "")
	t.Setenv("KAFKA_INITIAL_OFFSET", "invalid")
	t.Setenv("KAFKA_REBALANCE_TIMEOUT", "invalid")
	t.Setenv("KAFKA_REQUIRED_ACKS", "invalid")
	t.Setenv("KAFKA_REQUEST_TIMEOUT", "invalid")
	t.Setenv("DB_URL", "")
	cfg, err := LoadKafkaShareConsumer()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092"}) || cfg.Topic != "notifications.requested.v1" || cfg.GroupID != "shared-workers-v1" || cfg.MaxPollRecords != 100 || cfg.FetchMinBytes != 1 || cfg.FetchMaxBytes != 1000000 || cfg.FetchMaxWait != 500*time.Millisecond || cfg.RequestTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

// TestLoadKafkaShareConsumerOverrides verifies caller-selected buffering and
// normalized addresses without exposing SDK option types in configuration.
func TestLoadKafkaShareConsumerOverrides(t *testing.T) {
	setShareConsumerEnv(t)
	t.Setenv("KAFKA_BROKERS", " localhost:9092 , localhost:19092 ")
	t.Setenv("KAFKA_TOPIC", " jobs.v1 ")
	t.Setenv("KAFKA_SHARE_GROUP_ID", " shared-jobs ")
	t.Setenv("KAFKA_SHARE_MAX_POLL_RECORDS", " 5 ")
	t.Setenv("KAFKA_SHARE_FETCH_MIN_BYTES", "1024")
	t.Setenv("KAFKA_SHARE_FETCH_MAX_BYTES", "2000000")
	t.Setenv("KAFKA_SHARE_FETCH_MAX_WAIT", "1s")
	t.Setenv("KAFKA_SHARE_REQUEST_TIMEOUT", "2s")
	cfg, err := LoadKafkaShareConsumer()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Brokers, []string{"localhost:9092", "localhost:19092"}) || cfg.Topic != "jobs.v1" || cfg.GroupID != "shared-jobs" || cfg.MaxPollRecords != 5 || cfg.FetchMinBytes != 1024 || cfg.FetchMaxBytes != 2000000 || cfg.FetchMaxWait != time.Second || cfg.RequestTimeout != 2*time.Second {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}

// TestLoadKafkaShareConsumerValidation rejects malformed values and unsafe
// bounds at startup rather than silently falling back to library defaults.
func TestLoadKafkaShareConsumerValidation(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"KAFKA_BROKERS", ""}, {"KAFKA_TOPIC", " "}, {"KAFKA_SHARE_GROUP_ID", " "},
		{"KAFKA_SHARE_MAX_POLL_RECORDS", "invalid"}, {"KAFKA_SHARE_MAX_POLL_RECORDS", "0"}, {"KAFKA_SHARE_MAX_POLL_RECORDS", "10001"},
		{"KAFKA_SHARE_FETCH_MIN_BYTES", "0"}, {"KAFKA_SHARE_FETCH_MIN_BYTES", "1000001"},
		{"KAFKA_SHARE_FETCH_MAX_BYTES", "invalid"}, {"KAFKA_SHARE_FETCH_MAX_BYTES", "2147483648"},
		{"KAFKA_SHARE_FETCH_MAX_WAIT", "invalid"}, {"KAFKA_SHARE_FETCH_MAX_WAIT", "1ms"}, {"KAFKA_SHARE_FETCH_MAX_WAIT", "1000h"},
		{"KAFKA_SHARE_REQUEST_TIMEOUT", "invalid"}, {"KAFKA_SHARE_REQUEST_TIMEOUT", "1ms"}, {"KAFKA_SHARE_REQUEST_TIMEOUT", "16m"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			setShareConsumerEnv(t)
			t.Setenv(tc.key, tc.value)
			if _, err := LoadKafkaShareConsumer(); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error=%v want %s", err, tc.key)
			}
		})
	}
}

func setShareConsumerEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"KAFKA_BROKERS": "localhost:9092", "KAFKA_TOPIC": "jobs.v1", "KAFKA_SHARE_GROUP_ID": "shared-jobs",
		"KAFKA_SHARE_MAX_POLL_RECORDS": "100", "KAFKA_SHARE_FETCH_MIN_BYTES": "1", "KAFKA_SHARE_FETCH_MAX_BYTES": "1000000",
		"KAFKA_SHARE_FETCH_MAX_WAIT": "500ms", "KAFKA_SHARE_REQUEST_TIMEOUT": "10s",
	} {
		t.Setenv(key, value)
	}
}
