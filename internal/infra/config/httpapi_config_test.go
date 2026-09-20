package config

import (
	"strings"
	"testing"
)

func setValidHTTPAPIEnv(t *testing.T) {
	t.Helper()
	setValidRuntimeEnv(t)
	t.Setenv("HTTP_ADDRESS", ":8080")
	t.Setenv("HTTP_MAX_REQUEST_BODY_BYTES", "1048576")
	t.Setenv("HTTP_MAX_HEADER_BYTES", "16384")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "10s")
	t.Setenv("HTTP_CORS_ALLOW_ORIGINS", "http://localhost:3000")
	t.Setenv("HTTP_REQUEST_ID_INCOMING_HEADER", "X-Request-ID")
	t.Setenv("HTTP_REQUEST_ID_RESPONSE_HEADER", "X-Request-ID")
	t.Setenv("HTTP_REQUEST_ID_REJECT_INVALID", "false")
	t.Setenv("HEALTH_STREAM_CHECK_INTERVAL", "15s")
	t.Setenv("HEALTH_STREAM_HEARTBEAT_INTERVAL", "5s")
	t.Setenv("HEALTH_STREAM_MAX_DURATION", "1m")
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	t.Setenv("RATE_LIMIT_GLOBAL_IP_CAPACITY", "30")
	t.Setenv("RATE_LIMIT_GLOBAL_IP_REFILL_INTERVAL", "250ms")
}

// TestLoadHTTPAPIReadsLimitsAndRequestIDPolicy checks HTTP limits and request ID policy load from profile settings.
func TestLoadHTTPAPIReadsLimitsAndRequestIDPolicy(t *testing.T) {
	setValidHTTPAPIEnv(t)
	t.Setenv("HTTP_MAX_REQUEST_BODY_BYTES", "2097152")
	t.Setenv("HTTP_MAX_HEADER_BYTES", "32768")
	t.Setenv("HTTP_REQUEST_ID_INCOMING_HEADER", "  X-Correlation-ID  ")
	t.Setenv("HTTP_REQUEST_ID_RESPONSE_HEADER", "  X-Response-ID  ")
	t.Setenv("HTTP_REQUEST_ID_REJECT_INVALID", "true")

	cfg, err := LoadHTTPAPI()
	if err != nil {
		t.Fatalf("LoadHTTPAPI() error = %v", err)
	}
	if cfg.HTTP.MaxRequestBodyBytes != 2<<20 || cfg.HTTP.MaxHeaderBytes != 32<<10 {
		t.Fatalf("HTTP limits = (%d, %d)", cfg.HTTP.MaxRequestBodyBytes, cfg.HTTP.MaxHeaderBytes)
	}
	if cfg.HTTP.RequestID.IncomingKey != "X-Correlation-ID" || cfg.HTTP.RequestID.ResponseKey != "X-Response-ID" || !cfg.HTTP.RequestID.RejectInvalid {
		t.Fatalf("HTTP request ID config = %+v", cfg.HTTP.RequestID)
	}
}

// TestLoadHTTPAPIRejectsInvalidProfileSettings checks invalid HTTP settings fail before server startup.
func TestLoadHTTPAPIRejectsInvalidProfileSettings(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "address", key: "HTTP_ADDRESS", value: "   ", wantErr: "HTTP_ADDRESS must not be empty"},
		{name: "request body limit", key: "HTTP_MAX_REQUEST_BODY_BYTES", value: "0", wantErr: "HTTP_MAX_REQUEST_BODY_BYTES must be > 0"},
		{name: "header limit", key: "HTTP_MAX_HEADER_BYTES", value: "0", wantErr: "HTTP_MAX_HEADER_BYTES must be > 0"},
		{name: "request timeout", key: "HTTP_REQUEST_TIMEOUT", value: "0s", wantErr: "HTTP_REQUEST_TIMEOUT must be > 0"},
		{name: "CORS origins", key: "HTTP_CORS_ALLOW_ORIGINS", value: "   ", wantErr: "HTTP_CORS_ALLOW_ORIGINS must contain at least one origin"},
		{name: "wildcard CORS", key: "HTTP_CORS_ALLOW_ORIGINS", value: "*", wantErr: "HTTP_CORS_ALLOW_ORIGINS must not contain *"},
		{name: "rate limit", key: "RATE_LIMIT_GLOBAL_IP_CAPACITY", value: "0", wantErr: "RATE_LIMIT_GLOBAL_IP_CAPACITY"},
		{name: "health stream", key: "HEALTH_STREAM_MAX_DURATION", value: "15s", wantErr: "HEALTH_STREAM_MAX_DURATION must be greater"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidHTTPAPIEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := LoadHTTPAPI()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadHTTPAPI() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// TestLoadHTTPAPIRequiresSecureCookiesOutsideLocal checks nonlocal deployments cannot use insecure session cookies.
func TestLoadHTTPAPIRequiresSecureCookiesOutsideLocal(t *testing.T) {
	setValidHTTPAPIEnv(t)
	t.Setenv("APP_ENV", "production")

	_, err := LoadHTTPAPI()
	if err == nil || !strings.Contains(err.Error(), "SESSION_COOKIE_SECURE must be true") {
		t.Fatalf("LoadHTTPAPI() error = %v", err)
	}
}
