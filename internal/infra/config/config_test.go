package config

import (
	"strings"
	"testing"
	"time"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "local")
	t.Setenv("SERVICE_NAME", "app")
	t.Setenv("HTTP_ADDRESS", ":8080")
	t.Setenv("HTTP_READ_TIMEOUT", "10s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "15s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "60s")
	t.Setenv("HTTP_MAX_REQUEST_BODY_BYTES", "1048576")
	t.Setenv("HTTP_MAX_HEADER_BYTES", "16384")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "10s")
	t.Setenv("HTTP_CORS_ALLOW_ORIGINS", "http://localhost:3000")
	t.Setenv("HTTP_TRUSTED_PROXY_CIDRS", "")
	t.Setenv("HTTP_REQUEST_ID_INCOMING_HEADER", "X-Request-ID")
	t.Setenv("HTTP_REQUEST_ID_RESPONSE_HEADER", "X-Request-ID")
	t.Setenv("HTTP_REQUEST_ID_REJECT_INVALID", "false")
	t.Setenv("HEALTH_STREAM_CHECK_INTERVAL", "15s")
	t.Setenv("HEALTH_STREAM_HEARTBEAT_INTERVAL", "5s")
	t.Setenv("HEALTH_STREAM_MAX_DURATION", "1m")
	t.Setenv("GRPC_ADDRESS", ":9090")
	t.Setenv("GRPC_HEALTH_REFRESH_INTERVAL", "10s")
	t.Setenv("GRPC_REFLECTION_ENABLED", "true")
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
	t.Setenv("GRPC_CLIENT_TARGET", "localhost:9090")
	t.Setenv("GRPC_CLIENT_SERVICE_NAME", "grpcclient-demo")
	t.Setenv("GRPC_CLIENT_DEPENDENCY_NAME", "grpcapi-demo")
	t.Setenv("GRPC_CLIENT_REQUEST_TIMEOUT", "3s")
	t.Setenv("GRPC_CLIENT_MAX_RECV_MESSAGE_BYTES", "4194304")
	t.Setenv("GRPC_CLIENT_MAX_SEND_MESSAGE_BYTES", "4194304")
	t.Setenv("GRPC_CLIENT_REQUEST_ID_METADATA_KEY", "x-request-id")
	t.Setenv("GRPC_CLIENT_TRACE_PROPAGATION_ENABLED", "true")
	t.Setenv("GRPC_CLIENT_TLS_ENABLED", "false")
	t.Setenv("GRPC_CLIENT_TLS_SERVER_NAME", "")
	t.Setenv("GRPC_CLIENT_TLS_SERVER_CA_FILE", "")
	t.Setenv("GRPC_CLIENT_TLS_CERT_FILE", "")
	t.Setenv("GRPC_CLIENT_TLS_KEY_FILE", "")
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/app?sslmode=disable")
	t.Setenv("DB_MAX_CONNS", "10")
	t.Setenv("DB_MIN_CONNS", "2")
	t.Setenv("DB_MAX_CONN_LIFETIME", "30m")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "5m")
	t.Setenv("DB_HEALTH_CHECK_PERIOD", "1m")
	t.Setenv("DB_CONNECT_TIMEOUT", "5s")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "0")
	t.Setenv("REDIS_DIAL_TIMEOUT", "3s")
	t.Setenv("REDIS_READ_TIMEOUT", "2s")
	t.Setenv("REDIS_WRITE_TIMEOUT", "2s")
	t.Setenv("REDIS_CACHE_TTL", "2m")
	t.Setenv("OTEL_EXPORT_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_INSECURE", "true")
	t.Setenv("OTEL_TRACES_SAMPLER_RATIO", "1")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("OTEL_LOG_LEVEL", "info")
	t.Setenv("LOG_DEVELOPMENT", "true")
	t.Setenv("SHUTDOWN_GRACE_PERIOD", "10s")
	t.Setenv("RATE_LIMIT_GLOBAL_IP_CAPACITY", "30")
	t.Setenv("RATE_LIMIT_GLOBAL_IP_REFILL_INTERVAL", "250ms")
}

func TestLoad_HTTPSizeLimits(t *testing.T) {
	setValidEnv(t)
	t.Setenv("HTTP_MAX_REQUEST_BODY_BYTES", "2097152")
	t.Setenv("HTTP_MAX_HEADER_BYTES", "32768")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.MaxRequestBodyBytes != 2<<20 {
		t.Fatalf("MaxRequestBodyBytes = %d, want %d", cfg.HTTP.MaxRequestBodyBytes, 2<<20)
	}
	if cfg.HTTP.MaxHeaderBytes != 32<<10 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", cfg.HTTP.MaxHeaderBytes, 32<<10)
	}
}

func TestLoad_OTelExportDisabled(t *testing.T) {
	setValidEnv(t)
	t.Setenv("OTEL_EXPORT_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OTel.ExportEnabled {
		t.Fatal("ExportEnabled = true, want false")
	}
}

func TestLoad_RequestIDPolicies(t *testing.T) {
	setValidEnv(t)
	t.Setenv("HTTP_REQUEST_ID_INCOMING_HEADER", "  X-Correlation-ID  ")
	t.Setenv("HTTP_REQUEST_ID_RESPONSE_HEADER", "  X-Response-ID  ")
	t.Setenv("HTTP_REQUEST_ID_REJECT_INVALID", "true")
	t.Setenv("GRPC_REQUEST_ID_INCOMING_METADATA_KEY", "  correlation-id  ")
	t.Setenv("GRPC_REQUEST_ID_RESPONSE_METADATA_KEY", "  response-id  ")
	t.Setenv("GRPC_REQUEST_ID_REJECT_INVALID", "true")
	t.Setenv("GRPC_CLIENT_REQUEST_ID_METADATA_KEY", "  downstream-id  ")
	t.Setenv("GRPC_CLIENT_DEPENDENCY_NAME", "  diagnostics-service  ")
	t.Setenv("GRPC_CLIENT_SERVICE_NAME", "  diagnostics-cli  ")
	t.Setenv("GRPC_CLIENT_TRACE_PROPAGATION_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.RequestID.IncomingKey != "X-Correlation-ID" || cfg.HTTP.RequestID.ResponseKey != "X-Response-ID" || !cfg.HTTP.RequestID.RejectInvalid {
		t.Fatalf("HTTP request ID config = %+v", cfg.HTTP.RequestID)
	}
	if cfg.GRPC.RequestID.IncomingKey != "correlation-id" || cfg.GRPC.RequestID.ResponseKey != "response-id" || !cfg.GRPC.RequestID.RejectInvalid {
		t.Fatalf("gRPC request ID config = %+v", cfg.GRPC.RequestID)
	}
	if cfg.GRPCClient.RequestIDMetadataKey != "downstream-id" {
		t.Fatalf("gRPC client request ID key = %q", cfg.GRPCClient.RequestIDMetadataKey)
	}
	if cfg.GRPCClient.ServiceName != "diagnostics-cli" || cfg.GRPCClient.DependencyName != "diagnostics-service" || cfg.GRPCClient.TracePropagation {
		t.Fatalf("gRPC client observability config = %+v", cfg.GRPCClient)
	}
}

func TestLoad_GRPCRequestLimits(t *testing.T) {
	setValidEnv(t)
	t.Setenv("GRPC_REQUEST_TIMEOUT", "3s")
	t.Setenv("GRPC_MAX_RECV_MESSAGE_BYTES", "2097152")
	t.Setenv("GRPC_MAX_SEND_MESSAGE_BYTES", "1048576")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GRPC.RequestTimeout != 3*time.Second {
		t.Fatalf("RequestTimeout = %s, want 3s", cfg.GRPC.RequestTimeout)
	}
	if cfg.GRPC.MaxRecvMessageBytes != 2<<20 || cfg.GRPC.MaxSendMessageBytes != 1<<20 {
		t.Fatalf("gRPC message limits = (%d, %d)", cfg.GRPC.MaxRecvMessageBytes, cfg.GRPC.MaxSendMessageBytes)
	}
}

func TestLoad_GRPCTLS(t *testing.T) {
	setValidEnv(t)
	t.Setenv("GRPC_SERVER_TLS_ENABLED", "true")
	t.Setenv("GRPC_SERVER_TLS_CERT_FILE", "  /certs/server.pem  ")
	t.Setenv("GRPC_SERVER_TLS_KEY_FILE", "  /certs/server-key.pem  ")
	t.Setenv("GRPC_SERVER_TLS_CLIENT_CA_FILE", "  /certs/client-ca.pem  ")
	t.Setenv("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.GRPC.TLS.Enabled || !cfg.GRPC.TLS.RequireClientCert {
		t.Fatalf("gRPC TLS flags = %+v", cfg.GRPC.TLS)
	}
	if cfg.GRPC.TLS.ServerCertFile != "/certs/server.pem" || cfg.GRPC.TLS.ServerKeyFile != "/certs/server-key.pem" || cfg.GRPC.TLS.ClientCAFile != "/certs/client-ca.pem" {
		t.Fatalf("gRPC TLS paths = %+v", cfg.GRPC.TLS)
	}
}

func TestLoad_GRPCClient(t *testing.T) {
	setValidEnv(t)
	t.Setenv("GRPC_CLIENT_TARGET", "  dns:///diagnostics.internal:443  ")
	t.Setenv("GRPC_CLIENT_REQUEST_TIMEOUT", "5s")
	t.Setenv("GRPC_CLIENT_MAX_RECV_MESSAGE_BYTES", "2097152")
	t.Setenv("GRPC_CLIENT_MAX_SEND_MESSAGE_BYTES", "1048576")
	t.Setenv("GRPC_CLIENT_TLS_ENABLED", "true")
	t.Setenv("GRPC_CLIENT_TLS_SERVER_NAME", "  diagnostics.internal  ")
	t.Setenv("GRPC_CLIENT_TLS_SERVER_CA_FILE", "  /certs/root.pem  ")
	t.Setenv("GRPC_CLIENT_TLS_CERT_FILE", "  /certs/client.pem  ")
	t.Setenv("GRPC_CLIENT_TLS_KEY_FILE", "  /certs/client-key.pem  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GRPCClient.Target != "dns:///diagnostics.internal:443" || cfg.GRPCClient.RequestTimeout != 5*time.Second {
		t.Fatalf("gRPC client config = %+v", cfg.GRPCClient)
	}
	if cfg.GRPCClient.MaxRecvMessageBytes != 2<<20 || cfg.GRPCClient.MaxSendMessageBytes != 1<<20 {
		t.Fatalf("gRPC client message limits = (%d, %d)", cfg.GRPCClient.MaxRecvMessageBytes, cfg.GRPCClient.MaxSendMessageBytes)
	}
	if cfg.GRPCClient.TLS.ServerName != "diagnostics.internal" || cfg.GRPCClient.TLS.ServerCAFile != "/certs/root.pem" {
		t.Fatalf("gRPC client TLS config = %+v", cfg.GRPCClient.TLS)
	}
}

func TestLoad_RejectsInvalidGRPCClient(t *testing.T) {
	tests := []struct {
		name    string
		setEnv  func(*testing.T)
		wantErr string
	}{
		{name: "empty target", setEnv: func(t *testing.T) { t.Setenv("GRPC_CLIENT_TARGET", "   ") }, wantErr: "GRPC_CLIENT_TARGET must not be empty"},
		{name: "non-positive timeout", setEnv: func(t *testing.T) { t.Setenv("GRPC_CLIENT_REQUEST_TIMEOUT", "0s") }, wantErr: "GRPC_CLIENT_REQUEST_TIMEOUT must be > 0"},
		{name: "non-positive receive limit", setEnv: func(t *testing.T) { t.Setenv("GRPC_CLIENT_MAX_RECV_MESSAGE_BYTES", "0") }, wantErr: "GRPC_CLIENT_MAX_RECV_MESSAGE_BYTES"},
		{
			name: "incomplete mTLS identity",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_CLIENT_TLS_ENABLED", "true")
				t.Setenv("GRPC_CLIENT_TLS_CERT_FILE", "/certs/client.pem")
			},
			wantErr: "GRPC_CLIENT_TLS_CERT_FILE and GRPC_CLIENT_TLS_KEY_FILE must be configured together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			tt.setEnv(t)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_RejectsInvalidGRPCTLS(t *testing.T) {
	tests := []struct {
		name    string
		setEnv  func(*testing.T)
		wantErr string
	}{
		{
			name: "client certificate without TLS",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", "true")
			},
			wantErr: "GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT requires GRPC_SERVER_TLS_ENABLED",
		},
		{
			name: "missing server certificate",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_SERVER_TLS_ENABLED", "true")
				t.Setenv("GRPC_SERVER_TLS_KEY_FILE", "/certs/server-key.pem")
			},
			wantErr: "GRPC_SERVER_TLS_CERT_FILE and GRPC_SERVER_TLS_KEY_FILE are required",
		},
		{
			name: "missing server key",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_SERVER_TLS_ENABLED", "true")
				t.Setenv("GRPC_SERVER_TLS_CERT_FILE", "/certs/server.pem")
			},
			wantErr: "GRPC_SERVER_TLS_CERT_FILE and GRPC_SERVER_TLS_KEY_FILE are required",
		},
		{
			name: "missing client CA",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_SERVER_TLS_ENABLED", "true")
				t.Setenv("GRPC_SERVER_TLS_CERT_FILE", "/certs/server.pem")
				t.Setenv("GRPC_SERVER_TLS_KEY_FILE", "/certs/server-key.pem")
				t.Setenv("GRPC_SERVER_TLS_REQUIRE_CLIENT_CERT", "true")
			},
			wantErr: "GRPC_SERVER_TLS_CLIENT_CA_FILE is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			tt.setEnv(t)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_RejectsNonPositiveHTTPSizeLimits(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{name: "request body", key: "HTTP_MAX_REQUEST_BODY_BYTES", wantErr: "HTTP_MAX_REQUEST_BODY_BYTES must be > 0"},
		{name: "headers", key: "HTTP_MAX_HEADER_BYTES", wantErr: "HTTP_MAX_HEADER_BYTES must be > 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tt.key, "0")

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_RejectsWhitespaceOnlyRequiredStringFields(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{
			name:    "service name",
			key:     "SERVICE_NAME",
			wantErr: "SERVICE_NAME must not be empty",
		},
		{
			name:    "http address",
			key:     "HTTP_ADDRESS",
			wantErr: "HTTP_ADDRESS must not be empty",
		},
		{
			name:    "grpc address",
			key:     "GRPC_ADDRESS",
			wantErr: "GRPC_ADDRESS must not be empty",
		},
		{
			name:    "grpc client service name",
			key:     "GRPC_CLIENT_SERVICE_NAME",
			wantErr: "GRPC_CLIENT_SERVICE_NAME must not be empty",
		},
		{
			name:    "database url",
			key:     "DB_URL",
			wantErr: "DB_URL must not be empty",
		},
		{
			name:    "redis address",
			key:     "REDIS_ADDR",
			wantErr: "REDIS_ADDR must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tt.key, "   ")

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.key)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestLoad_TrimsRequiredStringFields(t *testing.T) {
	setValidEnv(t)
	t.Setenv("SERVICE_NAME", "  accounts-api  ")
	t.Setenv("HTTP_ADDRESS", "  :9090  ")
	t.Setenv("GRPC_ADDRESS", "  :9091  ")
	t.Setenv("DB_URL", "  postgres://postgres:postgres@localhost:5432/app?sslmode=disable  ")
	t.Setenv("REDIS_ADDR", "  localhost:6379  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.ServiceName != "accounts-api" {
		t.Fatalf("expected trimmed SERVICE_NAME, got %q", cfg.ServiceName)
	}
	if cfg.HTTP.Address != ":9090" {
		t.Fatalf("expected trimmed HTTP_ADDRESS, got %q", cfg.HTTP.Address)
	}
	if cfg.GRPC.Address != ":9091" {
		t.Fatalf("expected trimmed GRPC_ADDRESS, got %q", cfg.GRPC.Address)
	}
	if len(cfg.HTTP.CORSAllowOrigins) != 1 || cfg.HTTP.CORSAllowOrigins[0] != "http://localhost:3000" {
		t.Fatalf("CORS origins = %#v, want localhost origin", cfg.HTTP.CORSAllowOrigins)
	}
	if cfg.DB.URL != "postgres://postgres:postgres@localhost:5432/app?sslmode=disable" {
		t.Fatalf("expected trimmed DB_URL, got %q", cfg.DB.URL)
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("expected trimmed REDIS_ADDR, got %q", cfg.Redis.Addr)
	}
}

func TestLoad_HTTPAndProductionSafety(t *testing.T) {
	tests := []struct {
		name    string
		setEnv  func(*testing.T)
		wantErr string
	}{
		{
			name: "request timeout must be positive",
			setEnv: func(t *testing.T) {
				t.Setenv("HTTP_REQUEST_TIMEOUT", "0s")
			},
			wantErr: "HTTP_REQUEST_TIMEOUT must be > 0",
		},
		{
			name: "cors origins are required",
			setEnv: func(t *testing.T) {
				t.Setenv("HTTP_CORS_ALLOW_ORIGINS", "   ")
			},
			wantErr: "HTTP_CORS_ALLOW_ORIGINS must contain at least one origin",
		},
		{
			name: "wildcard CORS origin is unsafe with cookies",
			setEnv: func(t *testing.T) {
				t.Setenv("HTTP_CORS_ALLOW_ORIGINS", "*")
			},
			wantErr: "HTTP_CORS_ALLOW_ORIGINS must not contain * when cookies are enabled",
		},
		{
			name: "production requires secure session cookies",
			setEnv: func(t *testing.T) {
				t.Setenv("APP_ENV", "production")
				t.Setenv("SESSION_COOKIE_SECURE", "false")
			},
			wantErr: "SESSION_COOKIE_SECURE must be true when APP_ENV is not local",
		},
		{
			name: "global rate limit must be positive",
			setEnv: func(t *testing.T) {
				t.Setenv("RATE_LIMIT_GLOBAL_IP_CAPACITY", "0")
			},
			wantErr: "RATE_LIMIT_GLOBAL_IP_CAPACITY and RATE_LIMIT_GLOBAL_IP_REFILL_INTERVAL must be > 0",
		},
		{
			name: "health stream duration must include an update",
			setEnv: func(t *testing.T) {
				t.Setenv("HEALTH_STREAM_MAX_DURATION", "15s")
			},
			wantErr: "HEALTH_STREAM_MAX_DURATION must be greater than both",
		},
		{
			name: "grpc health refresh interval must be positive",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_HEALTH_REFRESH_INTERVAL", "0s")
			},
			wantErr: "GRPC_HEALTH_REFRESH_INTERVAL must be > 0",
		},
		{
			name: "grpc request timeout must be positive",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_REQUEST_TIMEOUT", "0s")
			},
			wantErr: "GRPC_REQUEST_TIMEOUT must be > 0",
		},
		{
			name: "grpc receive message limit must be positive",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_MAX_RECV_MESSAGE_BYTES", "0")
			},
			wantErr: "GRPC_MAX_RECV_MESSAGE_BYTES and GRPC_MAX_SEND_MESSAGE_BYTES must be > 0",
		},
		{
			name: "grpc send message limit must be positive",
			setEnv: func(t *testing.T) {
				t.Setenv("GRPC_MAX_SEND_MESSAGE_BYTES", "0")
			},
			wantErr: "GRPC_MAX_RECV_MESSAGE_BYTES and GRPC_MAX_SEND_MESSAGE_BYTES must be > 0",
		},
		{
			name: "shutdown grace period must be positive",
			setEnv: func(t *testing.T) {
				t.Setenv("SHUTDOWN_GRACE_PERIOD", "0s")
			},
			wantErr: "SHUTDOWN_GRACE_PERIOD must be > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			tt.setEnv(t)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
