package grpcapi

import (
	"strings"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/util/testutil"
)

func TestBuildTransportCredentials(t *testing.T) {
	t.Run("plaintext", func(t *testing.T) {
		transportCredentials, err := (wiring{}).buildTransportCredentials()
		if err != nil {
			t.Fatalf("buildTransportCredentials() error = %v", err)
		}
		if transportCredentials != nil {
			t.Fatal("plaintext transport credentials are not nil")
		}
	})

	t.Run("TLS", func(t *testing.T) {
		authority := testutil.NewCertificateAuthority(t)
		certificateFile, privateKeyFile := testutil.WriteCertificateFiles(t, "server", authority.IssueServerCertificate(t, "localhost"))
		wiring := wiring{cfg: config.Config{GRPC: config.GRPCConfig{TLS: config.GRPCTLSConfig{
			Enabled:         true,
			CertificateFile: certificateFile,
			PrivateKeyFile:  privateKeyFile,
		}}}}

		transportCredentials, err := wiring.buildTransportCredentials()
		if err != nil {
			t.Fatalf("buildTransportCredentials() error = %v", err)
		}
		if transportCredentials == nil {
			t.Fatal("TLS transport credentials are nil")
		}
	})

	t.Run("invalid certificate files", func(t *testing.T) {
		wiring := wiring{cfg: config.Config{GRPC: config.GRPCConfig{TLS: config.GRPCTLSConfig{
			Enabled:         true,
			CertificateFile: "missing.pem",
			PrivateKeyFile:  "missing-key.pem",
		}}}}

		_, err := wiring.buildTransportCredentials()
		if err == nil || !strings.Contains(err.Error(), "load gRPC server TLS configuration") {
			t.Fatalf("buildTransportCredentials() error = %v", err)
		}
	})
}
