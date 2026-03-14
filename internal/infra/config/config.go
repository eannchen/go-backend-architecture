package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration used by the process.
type Config struct {
	AppEnv      string
	ServiceName string
	HTTP        HTTPConfig
	DB          DBConfig
	Redis       RedisConfig
	Auth        AuthConfig
	OTel        OTelConfig
	Log         LogConfig
	Shutdown    ShutdownConfig
}

type AuthConfig struct {
	Session SessionConfig
	OTP     OTPConfig
	OAuth   OAuthConfig
}

type SessionConfig struct {
	TTL          time.Duration
	CookieName   string
	CookieSecure bool
}

type OTPConfig struct {
	TTL        time.Duration
	CodeLength int
}

type OAuthConfig struct {
	Google OAuthProviderConfig
}

type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type HTTPConfig struct {
	Address      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DBConfig struct {
	URL               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	CacheTTL     time.Duration
}

type LogConfig struct {
	Level       string
	OTELevel    string
	Development bool
}

type OTelConfig struct {
	Enabled            bool
	ExporterEndpoint   string
	TracesEndpoint     string
	LogsEndpoint       string
	MetricsEndpoint    string
	Insecure           bool
	TraceSamplingRatio float64
}

type ShutdownConfig struct {
	GracePeriod time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		ServiceName: getEnv("SERVICE_NAME", "app"),
		HTTP: HTTPConfig{
			Address:      getEnv("HTTP_ADDRESS", ":8080"),
			ReadTimeout:  getDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		DB: DBConfig{
			URL:               getEnv("DB_URL", "postgres://postgres:postgres@localhost:5432/app?sslmode=disable"),
			MaxConns:          int32(getInt("DB_MAX_CONNS", 10)),
			MinConns:          int32(getInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime:   getDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:   getDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthCheckPeriod: getDuration("DB_HEALTH_CHECK_PERIOD", time.Minute),
			ConnectTimeout:    getDuration("DB_CONNECT_TIMEOUT", 5*time.Second),
		},
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getInt("REDIS_DB", 0),
			DialTimeout:  getDuration("REDIS_DIAL_TIMEOUT", 3*time.Second),
			ReadTimeout:  getDuration("REDIS_READ_TIMEOUT", 2*time.Second),
			WriteTimeout: getDuration("REDIS_WRITE_TIMEOUT", 2*time.Second),
			CacheTTL:     getDuration("REDIS_CACHE_TTL", 2*time.Minute),
		},
		OTel: OTelConfig{
			Enabled:            getBool("OTEL_ENABLED", true),
			ExporterEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
			Insecure:           getBool("OTEL_INSECURE", true),
			TraceSamplingRatio: getFloat("OTEL_TRACES_SAMPLER_RATIO", 1.0),
		},
		Log: LogConfig{
			Level:       getEnv("LOG_LEVEL", "info"),
			OTELevel:    getEnv("OTEL_LOG_LEVEL", "info"),
			Development: getBool("LOG_DEVELOPMENT", true),
		},
		Auth: AuthConfig{
			Session: SessionConfig{
				TTL:          getDuration("SESSION_TTL", 24*time.Hour),
				CookieName:   getEnv("SESSION_COOKIE_NAME", "session"),
				CookieSecure: getBool("SESSION_COOKIE_SECURE", false),
			},
			OTP: OTPConfig{
				TTL:        getDuration("OTP_TTL", 5*time.Minute),
				CodeLength: getInt("OTP_CODE_LENGTH", 6),
			},
			OAuth: OAuthConfig{
				Google: OAuthProviderConfig{
					ClientID:     getEnv("OAUTH_GOOGLE_CLIENT_ID", ""),
					ClientSecret: getEnv("OAUTH_GOOGLE_CLIENT_SECRET", ""),
					RedirectURL:  getEnv("OAUTH_GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/oauth/google/callback"),
				},
			},
		},
		Shutdown: ShutdownConfig{
			GracePeriod: getDuration("SHUTDOWN_GRACE_PERIOD", 10*time.Second),
		},
	}

	cfg.ServiceName = strings.TrimSpace(cfg.ServiceName)
	cfg.HTTP.Address = strings.TrimSpace(cfg.HTTP.Address)
	cfg.DB.URL = strings.TrimSpace(cfg.DB.URL)
	cfg.Redis.Addr = strings.TrimSpace(cfg.Redis.Addr)
	cfg.OTel.ExporterEndpoint = strings.TrimSpace(cfg.OTel.ExporterEndpoint)

	if cfg.DB.URL == "" {
		return Config{}, fmt.Errorf("DB_URL must not be empty")
	}
	if cfg.DB.MinConns < 0 || cfg.DB.MaxConns < 1 || cfg.DB.MinConns > cfg.DB.MaxConns {
		return Config{}, fmt.Errorf("invalid DB pool configuration: min=%d max=%d", cfg.DB.MinConns, cfg.DB.MaxConns)
	}
	if cfg.Redis.Addr == "" {
		return Config{}, fmt.Errorf("REDIS_ADDR must not be empty")
	}
	if cfg.Redis.DB < 0 {
		return Config{}, fmt.Errorf("REDIS_DB must be >= 0")
	}
	if cfg.Redis.CacheTTL <= 0 {
		return Config{}, fmt.Errorf("REDIS_CACHE_TTL must be > 0")
	}
	if cfg.HTTP.Address == "" {
		return Config{}, fmt.Errorf("HTTP_ADDRESS must not be empty")
	}
	if cfg.ServiceName == "" {
		return Config{}, fmt.Errorf("SERVICE_NAME must not be empty")
	}
	if cfg.OTel.TraceSamplingRatio < 0 || cfg.OTel.TraceSamplingRatio > 1 {
		return Config{}, fmt.Errorf("OTEL_TRACES_SAMPLER_RATIO must be between 0 and 1")
	}
	cfg.OTel.TracesEndpoint = withOTLPPath(cfg.OTel.ExporterEndpoint, "/v1/traces")
	cfg.OTel.LogsEndpoint = withOTLPPath(cfg.OTel.ExporterEndpoint, "/v1/logs")
	cfg.OTel.MetricsEndpoint = withOTLPPath(cfg.OTel.ExporterEndpoint, "/v1/metrics")
	// Optional per-signal overrides if a collector exposes custom paths.
	if v := strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")); v != "" {
		cfg.OTel.TracesEndpoint = v
	}
	if v := strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")); v != "" {
		cfg.OTel.LogsEndpoint = v
	}
	if v := strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")); v != "" {
		cfg.OTel.MetricsEndpoint = v
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getBool(key string, fallback bool) bool {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getFloat(key string, fallback float64) float64 {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

func withOTLPPath(base, suffix string) string {
	trimmed := strings.TrimSpace(base)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return suffix
	}
	return trimmed + suffix
}
