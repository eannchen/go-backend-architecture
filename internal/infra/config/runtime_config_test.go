package config

import (
	"strings"
	"testing"
)

func setValidRuntimeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "local")
	t.Setenv("SERVICE_NAME", "app")
	t.Setenv("DB_URL", "postgres://postgres:postgres@localhost:5432/app?sslmode=disable")
	t.Setenv("DB_MAX_CONNS", "10")
	t.Setenv("DB_MIN_CONNS", "2")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("REDIS_DB", "0")
	t.Setenv("REDIS_CACHE_TTL", "2m")
	t.Setenv("OTEL_EXPORT_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_TRACES_SAMPLER_RATIO", "1")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("SHUTDOWN_GRACE_PERIOD", "10s")
}

// TestLoadRuntimeDisablesOnlyOTelExport checks disabling telemetry export leaves the rest of runtime configuration intact.
func TestLoadRuntimeDisablesOnlyOTelExport(t *testing.T) {
	setValidRuntimeEnv(t)
	t.Setenv("OTEL_EXPORT_ENABLED", "false")

	cfg, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if cfg.OTel.ExportEnabled {
		t.Fatal("ExportEnabled = true, want false")
	}
}

// TestLoadRuntimeTrimsRequiredFields checks required fields are trimmed before validation and use.
func TestLoadRuntimeTrimsRequiredFields(t *testing.T) {
	setValidRuntimeEnv(t)
	t.Setenv("SERVICE_NAME", "  accounts-api  ")
	t.Setenv("DB_URL", "  postgres://postgres:postgres@localhost:5432/app?sslmode=disable  ")
	t.Setenv("REDIS_ADDR", "  localhost:6379  ")

	cfg, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error = %v", err)
	}
	if cfg.ServiceName != "accounts-api" || cfg.DB.URL != "postgres://postgres:postgres@localhost:5432/app?sslmode=disable" || cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("LoadRuntime() did not trim required fields: %+v", cfg)
	}
}

// TestLoadRuntimeRejectsInvalidSharedSettings checks invalid shared settings fail at startup rather than during requests.
func TestLoadRuntimeRejectsInvalidSharedSettings(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "service name", key: "SERVICE_NAME", value: "   ", wantErr: "SERVICE_NAME must not be empty"},
		{name: "database URL", key: "DB_URL", value: "   ", wantErr: "DB_URL must not be empty"},
		{name: "Redis address", key: "REDIS_ADDR", value: "   ", wantErr: "REDIS_ADDR must not be empty"},
		{name: "shutdown grace period", key: "SHUTDOWN_GRACE_PERIOD", value: "0s", wantErr: "SHUTDOWN_GRACE_PERIOD must be > 0"},
		{name: "trace sampling ratio", key: "OTEL_TRACES_SAMPLER_RATIO", value: "2", wantErr: "OTEL_TRACES_SAMPLER_RATIO must be between 0 and 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidRuntimeEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := LoadRuntime()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
