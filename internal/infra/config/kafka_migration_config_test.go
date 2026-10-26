package config

import (
	"os"
	"strings"
	"testing"
)

// TestKafkaRemovedSettingsRejected prevents old client-specific tuning from
// appearing effective after migration; startup must name a supported alternative.
func TestKafkaRemovedSettingsRejected(t *testing.T) {
	for _, tc := range []struct {
		key       string
		publisher bool
	}{
		{"KAFKA_READ_TIMEOUT", true}, {"KAFKA_WRITE_TIMEOUT", true}, {"KAFKA_BATCH_SIZE", true},
		{"KAFKA_START_OFFSET", false}, {"KAFKA_READ_BATCH_TIMEOUT", false}, {"KAFKA_QUEUE_CAPACITY", false},
	} {
		t.Run(tc.key, func(t *testing.T) {
			for _, key := range []string{"KAFKA_READ_TIMEOUT", "KAFKA_WRITE_TIMEOUT", "KAFKA_BATCH_SIZE", "KAFKA_START_OFFSET", "KAFKA_READ_BATCH_TIMEOUT", "KAFKA_QUEUE_CAPACITY"} {
				t.Setenv(key, "")
				if err := os.Unsetenv(key); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv(tc.key, "1")
			var err error
			if tc.publisher {
				_, err = LoadKafkaPublisher()
			} else {
				_, err = LoadKafkaConsumer()
			}
			if err == nil || !strings.Contains(err.Error(), tc.key) || !strings.Contains(err.Error(), "no longer supported") {
				t.Fatalf("removed setting error=%v", err)
			}
		})
	}
}

// TestKafkaNewTuningValidation protects delivery budgets and integer/duration
// bounds before SDK options narrow values or reject configuration during wiring.
func TestKafkaNewTuningValidation(t *testing.T) {
	for _, tc := range []struct {
		key, value string
		publisher  bool
	}{
		{"KAFKA_DELIVERY_TIMEOUT", "invalid", true}, {"KAFKA_DELIVERY_TIMEOUT", "500ms", true},
		{"KAFKA_REQUEST_TIMEOUT", "1ns", true}, {"KAFKA_REQUEST_TIMEOUT", "16m", true},
		{"KAFKA_PRODUCE_TIMEOUT", "30d", true}, {"KAFKA_BATCH_BYTES", "2147483648", true},
		{"KAFKA_REQUEST_TIMEOUT", "1ns", false}, {"KAFKA_REBALANCE_TIMEOUT", "30d", false},
		{"KAFKA_MAX_CONCURRENT_FETCHES", "0", false}, {"KAFKA_FETCH_MAX_BYTES", "2147483648", false},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			setKafkaPublisherTuningDefaults(t)
			setKafkaConsumerFetchDefaults(t)
			t.Setenv("KAFKA_BROKERS", "localhost:9092")
			t.Setenv("KAFKA_TOPIC", "events.v1")
			t.Setenv("KAFKA_GROUP_ID", "workers")
			t.Setenv("KAFKA_REQUEST_TIMEOUT", "10s")
			t.Setenv("KAFKA_PRODUCE_TIMEOUT", "10s")
			t.Setenv(tc.key, tc.value)
			var err error
			if tc.publisher {
				_, err = LoadKafkaPublisher()
			} else {
				_, err = LoadKafkaConsumer()
			}
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("validation error=%v want %s", err, tc.key)
			}
		})
	}
}
