package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestKafkaSecurityConfiguration validates explicit modes without silently ignoring
// credentials or TLS material, and preserves opaque credential whitespace.
func TestKafkaSecurityConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name      string
		env       map[string]string
		wantError string
	}{
		{name: "local defaults"},
		{name: "TLS system roots", env: map[string]string{"KAFKA_TLS_ENABLED": "true"}},
		{name: "mutual TLS", env: map[string]string{"KAFKA_TLS_ENABLED": "true", "KAFKA_TLS_CLIENT_CERT_FILE": "client.pem", "KAFKA_TLS_CLIENT_KEY_FILE": "client.key"}},
		{name: "plain", env: map[string]string{"KAFKA_SASL_MECHANISM": "plain", "KAFKA_SASL_USERNAME": " user ", "KAFKA_SASL_PASSWORD": " secret "}},
		{name: "scram256", env: map[string]string{"KAFKA_SASL_MECHANISM": "SCRAM-SHA-256", "KAFKA_SASL_USERNAME": "user", "KAFKA_SASL_PASSWORD": "secret"}},
		{name: "scram512", env: map[string]string{"KAFKA_SASL_MECHANISM": "scram-sha-512", "KAFKA_SASL_USERNAME": "user", "KAFKA_SASL_PASSWORD": "secret"}},
		{name: "invalid bool", env: map[string]string{"KAFKA_TLS_ENABLED": "bad"}, wantError: "KAFKA_TLS_ENABLED"},
		{name: "ignored CA", env: map[string]string{"KAFKA_TLS_SERVER_CA_FILE": "root.pem"}, wantError: "KAFKA_TLS_ENABLED"},
		{name: "unpaired identity", env: map[string]string{"KAFKA_TLS_ENABLED": "true", "KAFKA_TLS_CLIENT_CERT_FILE": "client.pem"}, wantError: "configured together"},
		{name: "ignored credentials", env: map[string]string{"KAFKA_SASL_PASSWORD": "secret"}, wantError: "KAFKA_SASL_MECHANISM"},
		{name: "missing credentials", env: map[string]string{"KAFKA_SASL_MECHANISM": "plain"}, wantError: "KAFKA_SASL_USERNAME"},
		{name: "unknown mechanism", env: map[string]string{"KAFKA_SASL_MECHANISM": "unknown"}, wantError: "KAFKA_SASL_MECHANISM"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearKafkaSecurityEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			cfg, err := LoadKafkaSecurity()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || strings.Contains(err.Error(), "secret") {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "plain" && (cfg.SASL.Username != " user " || cfg.SASL.Password != " secret ") {
				t.Fatal("credentials were modified")
			}
		})
	}
}

// TestKafkaProfilesLoadSecurity protects publisher/consumer-independent loading
// while giving every profile the same connection security configuration.
func TestKafkaProfilesLoadSecurity(t *testing.T) {
	clearKafkaSecurityEnv(t)
	t.Setenv("KAFKA_TLS_ENABLED", "true")
	t.Setenv("KAFKA_SASL_MECHANISM", "scram-sha-512")
	t.Setenv("KAFKA_SASL_USERNAME", "alice")
	t.Setenv("KAFKA_SASL_PASSWORD", " password ")
	want, err := LoadKafkaSecurity()
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := LoadKafkaPublisher()
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := loadKafkaConsumer([]string{"events.v1"})
	if err != nil {
		t.Fatal(err)
	}
	share, err := loadKafkaShareConsumer([]string{"events.v1"})
	if err != nil {
		t.Fatal(err)
	}
	if publisher.Security != want || consumer.Security != want || share.Security != want {
		t.Fatal("profile security differs")
	}
}

func clearKafkaSecurityEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"KAFKA_TLS_ENABLED": "false", "KAFKA_TLS_SERVER_CA_FILE": "", "KAFKA_TLS_CLIENT_CERT_FILE": "", "KAFKA_TLS_CLIENT_KEY_FILE": "", "KAFKA_TLS_SERVER_NAME": "",
		"KAFKA_SASL_MECHANISM": "none", "KAFKA_SASL_USERNAME": "", "KAFKA_SASL_PASSWORD": "",
	} {
		t.Setenv(key, value)
	}
}

// TestWorkerRoutes checks topic bindings in both binaries, including subscribing
// to only one workflow, custom names, and startup rejection of ambiguous routes.
func TestWorkerRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, notification, activity string
		valid                        bool
	}{
		{"both", "notifications.requested.v1", "activities.recorded.v1", true},
		{"activity only", "", "activities.recorded.v1", true},
		{"renamed", " mail ", " actions ", true},
		{"duplicate binding", "mail", "mail", false},
		{"disabled activity", "mail", " ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setValidRuntimeEnv(t)
			t.Setenv("KAFKA_NOTIFICATION_TOPIC", tc.notification)
			t.Setenv("KAFKA_ACTIVITY_TOPIC", tc.activity)
			ordinary, err := LoadKafkaWorker()
			shared, shareErr := LoadKafkaShareWorker()
			if !tc.valid {
				if err == nil || shareErr == nil {
					t.Fatalf("invalid bindings accepted: %v / %v", err, shareErr)
				}
				return
			}
			if err != nil || shareErr != nil {
				t.Fatalf("load: %v / %v", err, shareErr)
			}
			if ordinary.Routes != shared.Routes {
				t.Fatal("worker bindings differ")
			}
		})
	}
}

// TestWorkerSubscriptionsDeriveFromBindings prevents repeated topic strings
// from being required and allows either example workflow to be disabled.
func TestWorkerSubscriptionsDeriveFromBindings(t *testing.T) {
	for _, tc := range []struct {
		notification, activity string
		count                  int
	}{
		{"mail", "actions", 2}, {"mail", "", 1}, {"", "actions", 1}, {"", "", 0},
	} {
		t.Setenv("KAFKA_NOTIFICATION_TOPIC", tc.notification)
		t.Setenv("KAFKA_ACTIVITY_TOPIC", tc.activity)
		routes, err := loadKafkaWorkerRoutes()
		if tc.count == 0 {
			if err == nil {
				t.Fatal("empty subscriptions accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		topics := routes.Topics()
		if len(topics) != tc.count {
			t.Fatalf("topics = %v", topics)
		}
		ordinary, err := loadKafkaConsumer(topics)
		if err != nil {
			t.Fatal(err)
		}
		shared, err := loadKafkaShareConsumer(topics)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ordinary.Topics, topics) || !slices.Equal(shared.Topics, topics) {
			t.Fatal("bindings lost")
		}
	}
}

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
				_, err = loadKafkaConsumer([]string{"events.v1"})
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
			t.Setenv("KAFKA_NOTIFICATION_TOPIC", "events.v1")
			t.Setenv("KAFKA_GROUP_ID", "workers")
			t.Setenv("KAFKA_REQUEST_TIMEOUT", "10s")
			t.Setenv("KAFKA_PRODUCE_TIMEOUT", "10s")
			t.Setenv(tc.key, tc.value)
			var err error
			if tc.publisher {
				_, err = LoadKafkaPublisher()
			} else {
				_, err = loadKafkaConsumer([]string{"events.v1"})
			}
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("validation error=%v want %s", err, tc.key)
			}
		})
	}
}
