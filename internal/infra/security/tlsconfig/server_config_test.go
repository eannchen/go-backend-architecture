package tlsconfig

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/util/testutil"
)

func TestLoadServerConfiguresTLSAndRequiredClientCertificates(t *testing.T) {
	authority := testutil.NewCertificateAuthority(t)
	serverCertificate := authority.IssueServerCertificate(t, "localhost")
	certificateFile, privateKeyFile := testutil.WriteCertificateFiles(t, "server", serverCertificate)

	tlsCfg, err := LoadServer(ServerConfig{
		CertificateFile:          certificateFile,
		PrivateKeyFile:           privateKeyFile,
		ClientCAFile:             authority.WriteCAFile(t),
		RequireClientCertificate: true,
	})
	if err != nil {
		t.Fatalf("LoadServer() error = %v", err)
	}
	if tlsCfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("minimum TLS version = %d, want TLS 1.2", tlsCfg.MinVersion)
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Fatalf("server certificates = %d, want 1", len(tlsCfg.Certificates))
	}
	if tlsCfg.ClientAuth != tls.RequireAndVerifyClientCert || tlsCfg.ClientCAs == nil {
		t.Fatalf("client authentication = %v, client CAs configured = %t", tlsCfg.ClientAuth, tlsCfg.ClientCAs != nil)
	}
}

func TestLoadServerVerifiesOptionalClientCertificates(t *testing.T) {
	authority := testutil.NewCertificateAuthority(t)
	certificateFile, privateKeyFile := testutil.WriteCertificateFiles(t, "server", authority.IssueServerCertificate(t, "localhost"))

	tlsCfg, err := LoadServer(ServerConfig{
		CertificateFile: certificateFile,
		PrivateKeyFile:  privateKeyFile,
		ClientCAFile:    authority.WriteCAFile(t),
	})
	if err != nil {
		t.Fatalf("LoadServer() error = %v", err)
	}
	if tlsCfg.ClientAuth != tls.VerifyClientCertIfGiven {
		t.Fatalf("client authentication = %v, want VerifyClientCertIfGiven", tlsCfg.ClientAuth)
	}
}

func TestLoadServerRejectsInvalidFiles(t *testing.T) {
	authority := testutil.NewCertificateAuthority(t)
	certificateFile, privateKeyFile := testutil.WriteCertificateFiles(t, "server", authority.IssueServerCertificate(t, "localhost"))
	invalidCAFile := filepath.Join(t.TempDir(), "invalid-ca.pem")
	if err := os.WriteFile(invalidCAFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write invalid CA: %v", err)
	}

	tests := []struct {
		name    string
		cfg     ServerConfig
		wantErr string
	}{
		{
			name:    "empty server identity",
			cfg:     ServerConfig{},
			wantErr: "server certificate and private key files are required",
		},
		{
			name: "missing server identity",
			cfg: ServerConfig{
				CertificateFile: "missing.pem",
				PrivateKeyFile:  "missing-key.pem",
			},
			wantErr: "load server certificate and private key",
		},
		{
			name: "required client certificate without CA",
			cfg: ServerConfig{
				CertificateFile:          certificateFile,
				PrivateKeyFile:           privateKeyFile,
				RequireClientCertificate: true,
			},
			wantErr: "client CA file is required",
		},
		{
			name: "invalid client CA",
			cfg: ServerConfig{
				CertificateFile: certificateFile,
				PrivateKeyFile:  privateKeyFile,
				ClientCAFile:    invalidCAFile,
			},
			wantErr: "no certificates found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadServer(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadServer() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
