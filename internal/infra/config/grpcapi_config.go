package config

import (
	"fmt"
	"strings"
	"time"
)

// GRPCAPIConfig combines shared runtime settings with the service gRPC profile.
type GRPCAPIConfig struct {
	RuntimeConfig
	GRPC GRPCConfig
}

type GRPCConfig struct {
	Address               string
	HealthRefreshInterval time.Duration
	ReflectionEnabled     bool
	RequestTimeout        time.Duration
	MaxRecvMessageBytes   int
	MaxSendMessageBytes   int
	RequestID             RequestIDConfig
	TLS                   GRPCServerTLSConfig
}

// GRPCServerTLSConfig controls server identity and optional client-certificate verification.
type GRPCServerTLSConfig struct {
	Enabled bool
	// ServerCertFile is the certificate chain the gRPC server presents to clients.
	ServerCertFile string
	// ServerKeyFile is the secret private key matching ServerCertFile.
	ServerKeyFile string
	// ClientCAFile identifies which client certificates the server trusts for mTLS.
	// Verified certificates must carry one URI SAN to become a caller identity.
	ClientCAFile string
	// RequireClientCert rejects clients that do not present a trusted certificate.
	RequireClientCert bool
}

func LoadGRPCAPI() (GRPCAPIConfig, error) {
	runtimeConfig, err := LoadRuntime()
	if err != nil {
		return GRPCAPIConfig{}, err
	}
	cfg := GRPCAPIConfig{
		RuntimeConfig: runtimeConfig,
		GRPC: GRPCConfig{
			Address:               getEnv("GRPC_ADDRESS", ":9090"),
			HealthRefreshInterval: getDuration("GRPC_HEALTH_REFRESH_INTERVAL", 10*time.Second),
			ReflectionEnabled:     getBool("GRPC_REFLECTION_ENABLED", isLocalAppEnv(runtimeConfig.AppEnv)),
			RequestTimeout:        getDuration("GRPC_REQUEST_TIMEOUT", 10*time.Second),
			MaxRecvMessageBytes:   getInt("GRPC_MAX_RECV_MESSAGE_BYTES", 4<<20),
			MaxSendMessageBytes:   getInt("GRPC_MAX_SEND_MESSAGE_BYTES", 4<<20),
			RequestID: RequestIDConfig{
				IncomingKey:   getEnv("GRPC_REQUEST_ID_INCOMING_METADATA_KEY", "x-request-id"),
				ResponseKey:   getEnv("GRPC_REQUEST_ID_RESPONSE_METADATA_KEY", "x-request-id"),
				RejectInvalid: getBool("GRPC_REQUEST_ID_REJECT_INVALID", false),
			},
			TLS: GRPCServerTLSConfig{
				Enabled:           getBool("GRPC_SERVER_TLS_ENABLED", false),
				ServerCertFile:    getEnv("GRPC_SERVER_TLS_CERT_FILE", ""),
				ServerKeyFile:     getEnv("GRPC_SERVER_TLS_KEY_FILE", ""),
				ClientCAFile:      getEnv("GRPC_SERVER_TLS_CLIENT_CA_FILE", ""),
				RequireClientCert: getBool("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", false),
			},
		},
	}

	cfg.GRPC.Address = strings.TrimSpace(cfg.GRPC.Address)
	cfg.GRPC.RequestID.IncomingKey = strings.TrimSpace(cfg.GRPC.RequestID.IncomingKey)
	cfg.GRPC.RequestID.ResponseKey = strings.TrimSpace(cfg.GRPC.RequestID.ResponseKey)
	cfg.GRPC.TLS.ServerCertFile = strings.TrimSpace(cfg.GRPC.TLS.ServerCertFile)
	cfg.GRPC.TLS.ServerKeyFile = strings.TrimSpace(cfg.GRPC.TLS.ServerKeyFile)
	cfg.GRPC.TLS.ClientCAFile = strings.TrimSpace(cfg.GRPC.TLS.ClientCAFile)

	if cfg.GRPC.Address == "" {
		return GRPCAPIConfig{}, fmt.Errorf("GRPC_ADDRESS must not be empty")
	}
	if cfg.GRPC.HealthRefreshInterval <= 0 {
		return GRPCAPIConfig{}, fmt.Errorf("GRPC_HEALTH_REFRESH_INTERVAL must be > 0")
	}
	if cfg.GRPC.RequestTimeout <= 0 {
		return GRPCAPIConfig{}, fmt.Errorf("GRPC_REQUEST_TIMEOUT must be > 0")
	}
	if cfg.GRPC.MaxRecvMessageBytes <= 0 || cfg.GRPC.MaxSendMessageBytes <= 0 {
		return GRPCAPIConfig{}, fmt.Errorf("GRPC_MAX_RECV_MESSAGE_BYTES and GRPC_MAX_SEND_MESSAGE_BYTES must be > 0")
	}
	if err := validateGRPCTLS(cfg.GRPC.TLS); err != nil {
		return GRPCAPIConfig{}, err
	}

	return cfg, nil
}

func validateGRPCTLS(cfg GRPCServerTLSConfig) error {
	if !cfg.Enabled {
		if cfg.RequireClientCert {
			return fmt.Errorf("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT requires GRPC_SERVER_TLS_ENABLED")
		}
		return nil
	}
	if cfg.ServerCertFile == "" || cfg.ServerKeyFile == "" {
		return fmt.Errorf("GRPC_SERVER_TLS_CERT_FILE and GRPC_SERVER_TLS_KEY_FILE are required when GRPC_SERVER_TLS_ENABLED is true")
	}
	if cfg.RequireClientCert && cfg.ClientCAFile == "" {
		return fmt.Errorf("GRPC_SERVER_TLS_CLIENT_CA_FILE is required when GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT is true")
	}
	return nil
}
