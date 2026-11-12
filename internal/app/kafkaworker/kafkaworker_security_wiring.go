package kafkaworker

import (
	"crypto/tls"
	"fmt"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig"
)

// Reuse the protocol-neutral TLS loader for trust roots and client certificates.
// Run before creating telemetry or clients so local file failures need no cleanup.
func buildTLSConfig(cfg config.KafkaTLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	tlsConfig, err := tlsconfig.LoadClient(tlsconfig.ClientConfig{
		ServerCAFile:   cfg.ServerCAFile,
		ClientCertFile: cfg.ClientCertFile,
		ClientKeyFile:  cfg.ClientKeyFile,
		ServerName:     cfg.ServerName,
	})
	if err != nil {
		return nil, fmt.Errorf("load Kafka client TLS configuration: %w", err)
	}
	return tlsConfig, nil
}
