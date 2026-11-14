package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig"
	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig/tlsconfigtest"

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

// TestKafkaTLSHandshake exercises real TLS handshakes using the exact TLS option
// injected into franz-go, protecting trust, hostname checks, and client identity.
func TestKafkaTLSHandshake(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		mutual, wrongCA, wrongName, missingClient bool
		wantFailure                               bool
	}{
		{name: "TLS"},
		{name: "mutual TLS", mutual: true},
		{name: "untrusted broker", wrongCA: true, wantFailure: true},
		{name: "wrong broker name", wrongName: true, wantFailure: true},
		{name: "missing client identity", mutual: true, missingClient: true, wantFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authority := tlsconfigtest.NewCertificateAuthority(t)
			serverCfg := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{authority.IssueServerCertificate(t, "localhost")}, SessionTicketsDisabled: true}
			loader := tlsconfig.ClientConfig{ServerCAFile: authority.WriteCAFile(t)}
			if tc.mutual {
				serverCfg.ClientAuth = tls.RequireAndVerifyClientCert
				serverCfg.ClientCAs = authority.CertPool()
				if !tc.missingClient {
					loader.ClientCertFile, loader.ClientKeyFile = tlsconfigtest.WriteCertificateFiles(t, "client", authority.IssueClientCertificate(t, "worker"))
				}
			}
			if tc.wrongCA {
				loader.ServerCAFile = tlsconfigtest.NewCertificateAuthority(t).WriteCAFile(t)
			}
			transport, err := tlsconfig.LoadClient(loader)
			if err != nil {
				t.Fatal(err)
			}
			opts, err := securityOptions(config.KafkaSecurityConfig{TLS: config.KafkaTLSConfig{Enabled: true}}, transport)
			if err != nil {
				t.Fatal(err)
			}
			client, err := kgo.NewClient(opts...)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			clientCfg := client.OptValue(kgo.DialTLSConfig).(*tls.Config).Clone()
			// Franz-go normally sets this to the host of each broker when dialing.
			clientCfg.ServerName = "localhost"
			if tc.wrongName {
				clientCfg.ServerName = "other.invalid"
			}
			left, right := net.Pipe()
			defer left.Close()
			defer right.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			server := tls.Server(right, serverCfg)
			done := make(chan error, 1)
			go func() { done <- server.HandshakeContext(ctx) }()
			clientConn := tls.Client(left, clientCfg)
			clientErr := clientConn.HandshakeContext(ctx)
			var readDone chan struct{}
			if clientErr != nil {
				if err := left.Close(); err != nil {
					t.Error(err)
				}
			} else {
				// TLS 1.3 can finish the client handshake before the server validates the
				// client certificate. Read its final alert so net.Pipe cannot block the server.
				readDone = make(chan struct{})
				go func() {
					defer close(readDone)
					var data [1]byte
					// The read only drains a handshake alert or our deliberate connection close.
					// No application data is exchanged, so either read result is expected.
					_, _ = clientConn.Read(data[:])
				}()
			}
			serverErr := <-done
			if err := left.Close(); err != nil {
				t.Error(err)
			}
			if readDone != nil {
				<-readDone
			}
			if errors.Is(clientErr, context.DeadlineExceeded) || errors.Is(serverErr, context.DeadlineExceeded) {
				t.Fatal("TLS test stalled")
			}
			if tc.wrongCA {
				var untrusted x509.UnknownAuthorityError
				if !errors.As(clientErr, &untrusted) {
					t.Fatalf("expected CA rejection: %v", clientErr)
				}
			}
			if tc.wrongName {
				var mismatch x509.HostnameError
				if !errors.As(clientErr, &mismatch) {
					t.Fatalf("expected hostname rejection: %v", clientErr)
				}
			}
			failed := clientErr != nil || serverErr != nil
			if failed != tc.wantFailure {
				t.Fatalf("handshake failure=%v want=%v; client=%v server=%v", failed, tc.wantFailure, clientErr, serverErr)
			}
			if !failed && tc.mutual && len(server.ConnectionState().VerifiedChains) == 0 {
				t.Fatal("client identity was not verified")
			}
		})
	}
}
