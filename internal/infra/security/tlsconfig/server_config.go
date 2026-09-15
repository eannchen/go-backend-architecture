package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// ServerConfig identifies server credentials and optional trusted client roots.
type ServerConfig struct {
	// ServerCertFile contains the certificate chain sent to clients so they can
	// verify the server's identity.
	ServerCertFile string
	// ServerKeyFile contains the private key matching ServerCertFile. It proves
	// ownership of the server certificate and must remain secret.
	ServerKeyFile string
	// ClientCAFile contains CA certificates used to verify certificates presented
	// by clients. The gRPC caller-identity adapter separately interprets a verified
	// certificate's URI SAN; this protocol-neutral loader does not choose identity.
	ClientCAFile string
	// RequireClientCert makes a verified client certificate mandatory. Together
	// with server authentication, this turns ordinary TLS into mutual TLS.
	RequireClientCert bool
}

// LoadServer loads server identity and optional client trust roots from PEM files.
func LoadServer(cfg ServerConfig) (*tls.Config, error) {
	if cfg.ServerCertFile == "" || cfg.ServerKeyFile == "" {
		return nil, fmt.Errorf("server certificate and private key files are required")
	}
	certificate, err := tls.LoadX509KeyPair(cfg.ServerCertFile, cfg.ServerKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server certificate and private key: %w", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}
	if cfg.ClientCAFile == "" {
		if cfg.RequireClientCert {
			return nil, fmt.Errorf("client CA file is required when client certificates are required")
		}
		return tlsCfg, nil
	}

	clientCAPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read client CA certificate: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
		return nil, fmt.Errorf("parse client CA certificate: no certificates found")
	}
	tlsCfg.ClientCAs = clientCAs
	if cfg.RequireClientCert {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	} else {
		// Validate a certificate when a client presents one without requiring
		// every caller to participate in mutual TLS.
		tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
	}
	return tlsCfg, nil
}
