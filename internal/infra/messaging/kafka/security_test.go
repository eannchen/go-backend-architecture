package kafka

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
)

// TestClientSecurityOptions protects identical TLS/SASL settings on all three
// clients and confirms explicit opt-in rather than credential-based inference.
func TestClientSecurityOptions(t *testing.T) {
	for _, mechanism := range []config.KafkaSASLMechanism{config.KafkaSASLNone, config.KafkaSASLPlain, config.KafkaSASLScramSHA256, config.KafkaSASLScramSHA512} {
		t.Run(string(mechanism), func(t *testing.T) {
			security := config.KafkaSecurityConfig{TLS: config.KafkaTLSConfig{Enabled: true}, SASL: config.KafkaSASLConfig{Mechanism: mechanism}}
			if mechanism != config.KafkaSASLNone {
				security.SASL.Username = "alice"
				security.SASL.Password = " secret "
			}
			tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
			writerCfg := validPublisherConfig()
			writerCfg.Security = security
			writer, err := NewWriter(writerCfg, tlsCfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := writer.Close(); err != nil {
					t.Error(err)
				}
			})
			readerCfg := validConsumerConfig()
			readerCfg.Security = security
			reader, err := NewReader(readerCfg, tlsCfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := reader.Close(ctx); err != nil {
					t.Error(err)
				}
			})
			shareCfg := validShareConfig()
			shareCfg.Security = security
			share, err := NewShareReader(shareCfg, tlsCfg)
			if err != nil {
				t.Fatal(err)
			}
			cleanupShareReader(t, share)
			for _, client := range []*kgo.Client{writer.client, reader.client, share.client} {
				actual, ok := client.OptValue(kgo.DialTLSConfig).(*tls.Config)
				if !ok || actual == nil || actual == tlsCfg || actual.MinVersion != tls.VersionTLS12 || actual.InsecureSkipVerify || actual.ServerName != "" {
					t.Fatal("TLS configuration was not isolated or hostname verification changed")
				}
				mechanisms := client.OptValue(kgo.SASL).([]sasl.Mechanism)
				if mechanism == config.KafkaSASLNone {
					if len(mechanisms) != 0 {
						t.Fatal("unexpected SASL")
					}
					continue
				}
				names := map[config.KafkaSASLMechanism]string{config.KafkaSASLPlain: "PLAIN", config.KafkaSASLScramSHA256: "SCRAM-SHA-256", config.KafkaSASLScramSHA512: "SCRAM-SHA-512"}
				if len(mechanisms) != 1 || mechanisms[0].Name() != names[mechanism] {
					t.Fatal("wrong SASL mechanism")
				}
				if mechanism == config.KafkaSASLPlain {
					_, initial, err := mechanisms[0].Authenticate(t.Context(), "broker")
					if err != nil || string(initial) != "\x00alice\x00 secret " {
						t.Fatal("credentials were modified")
					}
				}
			}
		})
	}
}

// TestSecurityRejectsMissingTLS prevents a secure config from silently creating
// a plaintext connection when app wiring forgets to inject transport credentials.
func TestSecurityRejectsMissingTLS(t *testing.T) {
	cfg := validPublisherConfig()
	cfg.Security.TLS.Enabled = true
	if writer, err := NewWriter(cfg, nil); err == nil || writer != nil {
		t.Fatal("missing TLS configuration accepted")
	}
	cfg.Security.TLS.Enabled = false
	if writer, err := NewWriter(cfg, &tls.Config{MinVersion: tls.VersionTLS12}); err == nil || writer != nil {
		t.Fatal("unexpected TLS configuration accepted")
	}
}
