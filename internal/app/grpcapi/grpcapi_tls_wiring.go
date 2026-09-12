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
		ServerCertFile:    d.cfg.GRPC.TLS.ServerCertFile,
		ServerKeyFile:     d.cfg.GRPC.TLS.ServerKeyFile,
		ClientCAFile:      d.cfg.GRPC.TLS.ClientCAFile,
		RequireClientCert: d.cfg.GRPC.TLS.RequireClientCert,
	})
	if err != nil {
		return nil, fmt.Errorf("load gRPC server TLS configuration: %w", err)
	}
	return credentials.NewTLS(tlsCfg), nil
}
