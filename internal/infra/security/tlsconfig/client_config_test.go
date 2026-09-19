package tlsconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig/tlsconfigtest"
)

func TestLoadClientLoadsTrustAndIdentity(t *testing.T) {
	authority := tlsconfigtest.NewCertificateAuthority(t)
	certificateFile, privateKeyFile := tlsconfigtest.WriteCertificateFiles(t, "client", authority.IssueClientCertificate(t, "diagnostics-client"))

	tlsCfg, err := LoadClient(ClientConfig{
		ServerName:     "localhost",
		ServerCAFile:   authority.WriteCAFile(t),
		ClientCertFile: certificateFile,
		ClientKeyFile:  privateKeyFile,
	})
	if err != nil {
		t.Fatalf("LoadClient() error = %v", err)
	}
	if tlsCfg.ServerName != "localhost" || tlsCfg.RootCAs == nil || len(tlsCfg.Certificates) != 1 {
		t.Fatalf("client TLS config = %+v", tlsCfg)
	}
}

func TestLoadClientRejectsIncompleteIdentity(t *testing.T) {
	_, err := LoadClient(ClientConfig{ClientCertFile: "/tmp/client.pem"})
	if err == nil || !strings.Contains(err.Error(), "configured together") {
		t.Fatalf("LoadClient() error = %v, want certificate-pair error", err)
	}
}

func TestLoadClientRejectsInvalidRootCA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid-ca.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write invalid CA: %v", err)
	}

	_, err := LoadClient(ClientConfig{ServerCAFile: path})
	if err == nil || !strings.Contains(err.Error(), "no certificates found") {
		t.Fatalf("LoadClient() error = %v, want parse error", err)
	}
}
