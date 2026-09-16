package config

import (
	"strings"
	"testing"
	"time"
)

func setValidGRPCAPIEnv(t *testing.T) {
	t.Helper()
	setValidRuntimeEnv(t)
	t.Setenv("GRPC_ADDRESS", ":9090")
	t.Setenv("GRPC_HEALTH_REFRESH_INTERVAL", "10s")
	t.Setenv("GRPC_REQUEST_TIMEOUT", "10s")
	t.Setenv("GRPC_MAX_RECV_MESSAGE_BYTES", "4194304")
	t.Setenv("GRPC_MAX_SEND_MESSAGE_BYTES", "4194304")
	t.Setenv("GRPC_REQUEST_ID_INCOMING_METADATA_KEY", "x-request-id")
	t.Setenv("GRPC_REQUEST_ID_RESPONSE_METADATA_KEY", "x-request-id")
	t.Setenv("GRPC_REQUEST_ID_REJECT_INVALID", "false")
	t.Setenv("GRPC_SERVER_TLS_ENABLED", "false")
	t.Setenv("GRPC_SERVER_TLS_CERT_FILE", "")
	t.Setenv("GRPC_SERVER_TLS_KEY_FILE", "")
	t.Setenv("GRPC_SERVER_TLS_CLIENT_CA_FILE", "")
	t.Setenv("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", "false")
}

func TestLoadGRPCAPIReadsLimitsRequestIDAndTLS(t *testing.T) {
	setValidGRPCAPIEnv(t)
	t.Setenv("GRPC_REQUEST_TIMEOUT", "3s")
	t.Setenv("GRPC_MAX_RECV_MESSAGE_BYTES", "2097152")
	t.Setenv("GRPC_MAX_SEND_MESSAGE_BYTES", "1048576")
	t.Setenv("GRPC_REQUEST_ID_INCOMING_METADATA_KEY", "  correlation-id  ")
	t.Setenv("GRPC_REQUEST_ID_RESPONSE_METADATA_KEY", "  response-id  ")
	t.Setenv("GRPC_REQUEST_ID_REJECT_INVALID", "true")
	t.Setenv("GRPC_SERVER_TLS_ENABLED", "true")
	t.Setenv("GRPC_SERVER_TLS_CERT_FILE", "  /certs/server.pem  ")
	t.Setenv("GRPC_SERVER_TLS_KEY_FILE", "  /certs/server-key.pem  ")
	t.Setenv("GRPC_SERVER_TLS_CLIENT_CA_FILE", "  /certs/client-ca.pem  ")
	t.Setenv("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", "true")

	cfg, err := LoadGRPCAPI()
	if err != nil {
		t.Fatalf("LoadGRPCAPI() error = %v", err)
	}
	if cfg.GRPC.RequestTimeout != 3*time.Second || cfg.GRPC.MaxRecvMessageBytes != 2<<20 || cfg.GRPC.MaxSendMessageBytes != 1<<20 {
		t.Fatalf("gRPC limits = %+v", cfg.GRPC)
	}
	if cfg.GRPC.RequestID.IncomingKey != "correlation-id" || cfg.GRPC.RequestID.ResponseKey != "response-id" || !cfg.GRPC.RequestID.RejectInvalid {
		t.Fatalf("gRPC request ID config = %+v", cfg.GRPC.RequestID)
	}
	if cfg.GRPC.TLS.ServerCertFile != "/certs/server.pem" || cfg.GRPC.TLS.ServerKeyFile != "/certs/server-key.pem" || cfg.GRPC.TLS.ClientCAFile != "/certs/client-ca.pem" {
		t.Fatalf("gRPC TLS config = %+v", cfg.GRPC.TLS)
	}
}

func TestLoadGRPCAPIRejectsInvalidProfileSettings(t *testing.T) {
	tests := []struct {
		name    string
		setEnv  func(*testing.T)
		wantErr string
	}{
		{name: "address", setEnv: func(t *testing.T) { t.Setenv("GRPC_ADDRESS", "   ") }, wantErr: "GRPC_ADDRESS must not be empty"},
		{name: "health interval", setEnv: func(t *testing.T) { t.Setenv("GRPC_HEALTH_REFRESH_INTERVAL", "0s") }, wantErr: "GRPC_HEALTH_REFRESH_INTERVAL must be > 0"},
		{name: "request timeout", setEnv: func(t *testing.T) { t.Setenv("GRPC_REQUEST_TIMEOUT", "0s") }, wantErr: "GRPC_REQUEST_TIMEOUT must be > 0"},
		{name: "message limits", setEnv: func(t *testing.T) { t.Setenv("GRPC_MAX_RECV_MESSAGE_BYTES", "0") }, wantErr: "GRPC_MAX_RECV_MESSAGE_BYTES"},
		{name: "mTLS without TLS", setEnv: func(t *testing.T) { t.Setenv("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", "true") }, wantErr: "requires GRPC_SERVER_TLS_ENABLED"},
		{name: "missing server certificate", setEnv: func(t *testing.T) {
			t.Setenv("GRPC_SERVER_TLS_ENABLED", "true")
			t.Setenv("GRPC_SERVER_TLS_KEY_FILE", "/key.pem")
		}, wantErr: "GRPC_SERVER_TLS_CERT_FILE and GRPC_SERVER_TLS_KEY_FILE"},
		{name: "missing client CA", setEnv: func(t *testing.T) {
			t.Setenv("GRPC_SERVER_TLS_ENABLED", "true")
			t.Setenv("GRPC_SERVER_TLS_CERT_FILE", "/cert.pem")
			t.Setenv("GRPC_SERVER_TLS_KEY_FILE", "/key.pem")
			t.Setenv("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", "true")
		}, wantErr: "GRPC_SERVER_TLS_CLIENT_CA_FILE is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidGRPCAPIEnv(t)
			tt.setEnv(t)

			_, err := LoadGRPCAPI()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadGRPCAPI() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
