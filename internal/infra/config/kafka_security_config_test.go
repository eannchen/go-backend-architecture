package config

import (
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
	consumer, err := LoadKafkaConsumer()
	if err != nil {
		t.Fatal(err)
	}
	share, err := LoadKafkaShareConsumer()
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
