package config

import (
	"strings"
	"testing"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "local")
	t.Setenv("SERVICE_NAME", "app")
	t.Setenv("HTTP_ADDRESS", ":8080")
	t.Setenv("HTTP_READ_TIMEOUT", "10s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "15s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "60s")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "10s")
	t.Setenv("HTTP_CORS_ALLOW_ORIGINS", "http://localhost:3000")
	t.Setenv("HTTP_TRUSTED_PROXY_CIDRS", "")
	t.Setenv("HEALTH_STREAM_CHECK_INTERVAL", "15s")
	t.Setenv("HEALTH_STREAM_HEARTBEAT_INTERVAL", "5s")
	t.Setenv("HEALTH_STREAM_MAX_DURATION", "1m")
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
	t.Setenv("OTEL_ENABLED", "true")
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
			name: "production requires secure session cookies",
			setEnv: func(t *testing.T) {
				t.Setenv("APP_ENV", "production")
				t.Setenv("SESSION_COOKIE_SECURE", "false")
			},
			wantErr: "SESSION_COOKIE_SECURE must be true when APP_ENV is not local",
		},
		{
			name: "health stream duration must include an update",
			setEnv: func(t *testing.T) {
				t.Setenv("HEALTH_STREAM_MAX_DURATION", "15s")
			},
			wantErr: "HEALTH_STREAM_MAX_DURATION must be greater than both",
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
