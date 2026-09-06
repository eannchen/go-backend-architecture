package grpcapi

import (
	"fmt"

	"google.golang.org/grpc/credentials"

	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig"
)

func (d wiring) buildTransportCredentials() (credentials.TransportCredentials, error) {
	if !d.cfg.GRPC.TLS.Enabled {
		return nil, nil
	}
	tlsCfg, err := tlsconfig.LoadServer(tlsconfig.ServerConfig{
		CertificateFile:          d.cfg.GRPC.TLS.CertificateFile,
		PrivateKeyFile:           d.cfg.GRPC.TLS.PrivateKeyFile,
		ClientCAFile:             d.cfg.GRPC.TLS.ClientCAFile,
		RequireClientCertificate: d.cfg.GRPC.TLS.RequireClientCertificate,
	})
	if err != nil {
		return nil, fmt.Errorf("load gRPC server TLS configuration: %w", err)
	}
	return credentials.NewTLS(tlsCfg), nil
}
