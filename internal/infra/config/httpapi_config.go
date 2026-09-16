package config

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// HTTPAPIConfig combines shared runtime settings with the public HTTP API profile.
type HTTPAPIConfig struct {
	RuntimeConfig
	HTTP      HTTPConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
}

type RateLimitConfig struct {
	GlobalIPCapacity       int
	GlobalIPRefillInterval time.Duration
}

type AuthConfig struct {
	Session SessionConfig
	OTP     OTPConfig
	OAuth   OAuthConfig
	Resend  ResendConfig
}

// ResendConfig holds settings for Resend email API (OTP).
type ResendConfig struct {
	APIKey string
	From   string
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

// OAuthConfig holds per-provider settings. Google is used by oauth/login/google (Sign in with Google).
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
	// MaxRequestBodyBytes and MaxHeaderBytes bound untrusted HTTP input before a
	// handler allocates or parses it.
	MaxRequestBodyBytes int64
	MaxHeaderBytes      int
	RequestTimeout      time.Duration
	CORSAllowOrigins    []string
	TrustedProxyCIDRs   []string
	RequestID           RequestIDConfig
	HealthStream        HealthStreamConfig
}

// HealthStreamConfig bounds the health SSE demonstration endpoint.
type HealthStreamConfig struct {
	CheckInterval     time.Duration
	HeartbeatInterval time.Duration
	MaxDuration       time.Duration
}

func LoadHTTPAPI() (HTTPAPIConfig, error) {
	runtimeConfig, err := LoadRuntime()
	if err != nil {
		return HTTPAPIConfig{}, err
	}
	cfg := HTTPAPIConfig{
		RuntimeConfig: runtimeConfig,
		HTTP: HTTPConfig{
			Address:             getEnv("HTTP_ADDRESS", ":8080"),
			ReadTimeout:         getDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:        getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:         getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			MaxRequestBodyBytes: int64(getInt("HTTP_MAX_REQUEST_BODY_BYTES", 1<<20)),
			MaxHeaderBytes:      getInt("HTTP_MAX_HEADER_BYTES", 16<<10),
			RequestTimeout:      getDuration("HTTP_REQUEST_TIMEOUT", 10*time.Second),
			CORSAllowOrigins:    getCSV("HTTP_CORS_ALLOW_ORIGINS", []string{"http://localhost:3000"}),
			TrustedProxyCIDRs:   getCSV("HTTP_TRUSTED_PROXY_CIDRS", nil),
			RequestID: RequestIDConfig{
				IncomingKey:   getEnv("HTTP_REQUEST_ID_INCOMING_HEADER", "X-Request-ID"),
				ResponseKey:   getEnv("HTTP_REQUEST_ID_RESPONSE_HEADER", "X-Request-ID"),
				RejectInvalid: getBool("HTTP_REQUEST_ID_REJECT_INVALID", false),
			},
			HealthStream: HealthStreamConfig{
				CheckInterval:     getDuration("HEALTH_STREAM_CHECK_INTERVAL", 15*time.Second),
				HeartbeatInterval: getDuration("HEALTH_STREAM_HEARTBEAT_INTERVAL", 5*time.Second),
				MaxDuration:       getDuration("HEALTH_STREAM_MAX_DURATION", time.Minute),
			},
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
			Resend: ResendConfig{
				APIKey: getEnv("RESEND_API_KEY", ""),
				From:   getEnv("RESEND_FROM", ""),
			},
		},
		RateLimit: RateLimitConfig{
			GlobalIPCapacity:       getInt("RATE_LIMIT_GLOBAL_IP_CAPACITY", 30),
			GlobalIPRefillInterval: getDuration("RATE_LIMIT_GLOBAL_IP_REFILL_INTERVAL", 250*time.Millisecond),
		},
	}

	cfg.HTTP.Address = strings.TrimSpace(cfg.HTTP.Address)
	cfg.HTTP.RequestID.IncomingKey = strings.TrimSpace(cfg.HTTP.RequestID.IncomingKey)
	cfg.HTTP.RequestID.ResponseKey = strings.TrimSpace(cfg.HTTP.RequestID.ResponseKey)

	if cfg.HTTP.Address == "" {
		return HTTPAPIConfig{}, fmt.Errorf("HTTP_ADDRESS must not be empty")
	}
	if cfg.HTTP.MaxRequestBodyBytes <= 0 {
		return HTTPAPIConfig{}, fmt.Errorf("HTTP_MAX_REQUEST_BODY_BYTES must be > 0")
	}
	if cfg.HTTP.MaxHeaderBytes <= 0 {
		return HTTPAPIConfig{}, fmt.Errorf("HTTP_MAX_HEADER_BYTES must be > 0")
	}
	if cfg.HTTP.RequestTimeout <= 0 {
		return HTTPAPIConfig{}, fmt.Errorf("HTTP_REQUEST_TIMEOUT must be > 0")
	}
	if len(cfg.HTTP.CORSAllowOrigins) == 0 {
		return HTTPAPIConfig{}, fmt.Errorf("HTTP_CORS_ALLOW_ORIGINS must contain at least one origin")
	}
	if slices.Contains(cfg.HTTP.CORSAllowOrigins, "*") {
		return HTTPAPIConfig{}, fmt.Errorf("HTTP_CORS_ALLOW_ORIGINS must not contain * when cookies are enabled")
	}
	if cfg.HTTP.HealthStream.CheckInterval <= 0 || cfg.HTTP.HealthStream.HeartbeatInterval <= 0 {
		return HTTPAPIConfig{}, fmt.Errorf("HEALTH_STREAM_CHECK_INTERVAL and HEALTH_STREAM_HEARTBEAT_INTERVAL must be > 0")
	}
	if cfg.HTTP.HealthStream.MaxDuration <= cfg.HTTP.HealthStream.CheckInterval || cfg.HTTP.HealthStream.MaxDuration <= cfg.HTTP.HealthStream.HeartbeatInterval {
		return HTTPAPIConfig{}, fmt.Errorf("HEALTH_STREAM_MAX_DURATION must be greater than both HEALTH_STREAM_CHECK_INTERVAL and HEALTH_STREAM_HEARTBEAT_INTERVAL")
	}
	if cfg.RateLimit.GlobalIPCapacity <= 0 || cfg.RateLimit.GlobalIPRefillInterval <= 0 {
		return HTTPAPIConfig{}, fmt.Errorf("RATE_LIMIT_GLOBAL_IP_CAPACITY and RATE_LIMIT_GLOBAL_IP_REFILL_INTERVAL must be > 0")
	}
	if !isLocalAppEnv(cfg.AppEnv) && !cfg.Auth.Session.CookieSecure {
		return HTTPAPIConfig{}, fmt.Errorf("SESSION_COOKIE_SECURE must be true when APP_ENV is not local")
	}

	return cfg, nil
}
