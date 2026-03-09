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
	if cfg.DB.URL != "postgres://postgres:postgres@localhost:5432/app?sslmode=disable" {
		t.Fatalf("expected trimmed DB_URL, got %q", cfg.DB.URL)
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("expected trimmed REDIS_ADDR, got %q", cfg.Redis.Addr)
	}
}
