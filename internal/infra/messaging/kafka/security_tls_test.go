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
)

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
