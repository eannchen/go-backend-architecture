package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// RuntimeConfig holds infrastructure settings shared by every process profile.
type RuntimeConfig struct {
	AppEnv      string
	ServiceName string
	DB          DBConfig
	Redis       RedisConfig
	OTel        OTelConfig
	Log         LogConfig
	Shutdown    ShutdownConfig
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
	// ExportEnabled controls collector traffic; local instrumentation and trace propagation remain active.
	ExportEnabled      bool
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

// RequestIDConfig controls optional request-ID interoperability for an inbound transport.
type RequestIDConfig struct {
	IncomingKey   string
	ResponseKey   string
	RejectInvalid bool
}

func LoadRuntime() (RuntimeConfig, error) {
	cfg := RuntimeConfig{
		AppEnv:      getEnv("APP_ENV", "local"),
		ServiceName: getEnv("SERVICE_NAME", "app"),
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
			ExportEnabled:      getBool("OTEL_EXPORT_ENABLED", true),
			ExporterEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
			Insecure:           getBool("OTEL_INSECURE", true),
			TraceSamplingRatio: getFloat("OTEL_TRACES_SAMPLER_RATIO", 1.0),
		},
		Log: LogConfig{
			Level:       getEnv("LOG_LEVEL", "info"),
			OTELevel:    getEnv("OTEL_LOG_LEVEL", "info"),
			Development: getBool("LOG_DEVELOPMENT", true),
		},
		Shutdown: ShutdownConfig{
			GracePeriod: getDuration("SHUTDOWN_GRACE_PERIOD", 10*time.Second),
		},
	}

	cfg.AppEnv = strings.TrimSpace(cfg.AppEnv)
	cfg.ServiceName = strings.TrimSpace(cfg.ServiceName)
	cfg.DB.URL = strings.TrimSpace(cfg.DB.URL)
	cfg.Redis.Addr = strings.TrimSpace(cfg.Redis.Addr)
	cfg.OTel.ExporterEndpoint = strings.TrimSpace(cfg.OTel.ExporterEndpoint)

	if cfg.AppEnv == "" {
		return RuntimeConfig{}, fmt.Errorf("APP_ENV must not be empty")
	}
	if cfg.ServiceName == "" {
		return RuntimeConfig{}, fmt.Errorf("SERVICE_NAME must not be empty")
	}
	if cfg.DB.URL == "" {
		return RuntimeConfig{}, fmt.Errorf("DB_URL must not be empty")
	}
	if cfg.DB.MinConns < 0 || cfg.DB.MaxConns < 1 || cfg.DB.MinConns > cfg.DB.MaxConns {
		return RuntimeConfig{}, fmt.Errorf("invalid DB pool configuration: min=%d max=%d", cfg.DB.MinConns, cfg.DB.MaxConns)
	}
	if cfg.Redis.Addr == "" {
		return RuntimeConfig{}, fmt.Errorf("REDIS_ADDR must not be empty")
	}
	if cfg.Redis.DB < 0 {
		return RuntimeConfig{}, fmt.Errorf("REDIS_DB must be >= 0")
	}
	if cfg.Redis.CacheTTL <= 0 {
		return RuntimeConfig{}, fmt.Errorf("REDIS_CACHE_TTL must be > 0")
	}
	if cfg.Shutdown.GracePeriod <= 0 {
		return RuntimeConfig{}, fmt.Errorf("SHUTDOWN_GRACE_PERIOD must be > 0")
	}
	if cfg.OTel.TraceSamplingRatio < 0 || cfg.OTel.TraceSamplingRatio > 1 {
		return RuntimeConfig{}, fmt.Errorf("OTEL_TRACES_SAMPLER_RATIO must be between 0 and 1")
	}

	cfg.OTel.TracesEndpoint = withOTLPPath(cfg.OTel.ExporterEndpoint, "/v1/traces")
	cfg.OTel.LogsEndpoint = withOTLPPath(cfg.OTel.ExporterEndpoint, "/v1/logs")
	cfg.OTel.MetricsEndpoint = withOTLPPath(cfg.OTel.ExporterEndpoint, "/v1/metrics")
	// Optional per-signal overrides support collectors that expose custom paths.
	if value := strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")); value != "" {
		cfg.OTel.TracesEndpoint = value
	}
	if value := strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")); value != "" {
		cfg.OTel.LogsEndpoint = value
	}
	if value := strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")); value != "" {
		cfg.OTel.MetricsEndpoint = value
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getBool(key string, fallback bool) bool {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getCSV(key string, fallback []string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	values := strings.Split(raw, ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func isLocalAppEnv(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "local", "dev", "development", "test":
		return true
	default:
		return false
	}
}

func getFloat(key string, fallback float64) float64 {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func withOTLPPath(base, suffix string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed == "" {
		return suffix
	}
	return trimmed + suffix
}
