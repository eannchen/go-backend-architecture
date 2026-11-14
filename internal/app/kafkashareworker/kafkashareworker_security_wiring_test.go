package kafkashareworker

import (
	"crypto/tls"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig/tlsconfigtest"
)

// TestBuildTLSConfig loads the app's trust and identity through the shared loader,
// retaining broker hostname verification and rejecting local material failures.
func TestBuildTLSConfig(t *testing.T) {
	if cfg, err := buildTLSConfig(config.KafkaTLSConfig{}); err != nil || cfg != nil {
		t.Fatal("disabled TLS must not create transport credentials")
	}
	authority := tlsconfigtest.NewCertificateAuthority(t)
	cert, key := tlsconfigtest.WriteCertificateFiles(t, "client", authority.IssueClientCertificate(t, "worker"))
	security := config.KafkaSecurityConfig{TLS: config.KafkaTLSConfig{Enabled: true, ServerCAFile: authority.WriteCAFile(t), ClientCertFile: cert, ClientKeyFile: key}}
	cfg, err := buildTLSConfig(security.TLS)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinVersion != tls.VersionTLS12 || cfg.InsecureSkipVerify || cfg.ServerName != "" || cfg.RootCAs == nil || len(cfg.Certificates) != 1 {
		t.Fatal("TLS trust or identity was lost")
	}
	security.TLS.ServerCAFile = "missing-root.pem"
	if _, err := buildTLSConfig(security.TLS); err == nil {
		t.Fatal("missing CA was ignored")
	}
}
