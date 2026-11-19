package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// ClientConfig identifies trusted server roots and optional client identity.
type ClientConfig struct {
	// ServerName is the DNS name or IP identity expected in the server
	// certificate. It can differ from the network address used to connect.
	ServerName string
	// ServerCAFile contains CA certificates trusted when verifying the server.
	// When empty, Go uses the operating system's trusted root certificates.
	ServerCAFile string
	// ClientCertFile contains the certificate chain presented to the server
	// during mTLS. Configure it together with ClientKeyFile.
	ClientCertFile string
	// ClientKeyFile contains the private key matching ClientCertFile. It proves
	// the client's identity to an mTLS server and must remain secret.
	ClientKeyFile string
}

// LoadClient loads client trust roots and optional mutual-TLS credentials.
func LoadClient(cfg ClientConfig) (*tls.Config, error) {
	if (cfg.ClientCertFile == "") != (cfg.ClientKeyFile == "") {
		return nil, fmt.Errorf("client certificate and private key files must be configured together")
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.ServerName,
	}
	if cfg.ServerCAFile != "" {
		rootCAPEM, err := os.ReadFile(cfg.ServerCAFile)
		if err != nil {
			return nil, fmt.Errorf("read server CA certificate: %w", err)
		}
		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(rootCAPEM) {
			return nil, fmt.Errorf("parse server CA certificate: no certificates found")
		}
		tlsCfg.RootCAs = rootCAs
	}

	if cfg.ClientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate and private key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{certificate}
	}
	return tlsCfg, nil
}
