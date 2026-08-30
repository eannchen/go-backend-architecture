package api

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/repository/kvstore/kvstoretest"
)

func TestBuildServerAppliesEnvironmentHTTPProtection(t *testing.T) {
	tests := []struct {
		name     string
		appEnv   string
		wantHSTS bool
	}{
		{name: "production enables HSTS", appEnv: "production", wantHSTS: true},
		{name: "development omits HSTS", appEnv: "development", wantHSTS: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wiring := newWiring(config.Config{
				AppEnv: tt.appEnv,
				HTTP: config.HTTPConfig{
					Address:          ":0",
					ReadTimeout:      time.Second,
					WriteTimeout:     time.Second,
					IdleTimeout:      time.Second,
					RequestTimeout:   time.Second,
					CORSAllowOrigins: []string{"https://app.example.com"},
				},
			}, logger.NoopLogger{}, observability.NoopTracer{}, observability.NoopMeter{})

			tokenBucket := &kvstoretest.TokenBucketRepository{
				AllowFunc: func(context.Context, string, int, time.Duration) (repokvstore.TokenBucketDecision, error) {
					return repokvstore.TokenBucketDecision{Allowed: true}, nil
				},
			}
			server, err := wiring.buildServer(httpresponse.NewResponder(httpcontext.NewContextMeta()), appRepositories{tokenBucketRepo: tokenBucket}, appHandlers{}, appUsecases{})
			if err != nil {
				t.Fatalf("buildServer() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/missing", nil)
			req.TLS = &tls.ConnectionState{}
			req.Header.Set(echo.HeaderOrigin, "https://app.example.com")
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got := rec.Header().Get("Strict-Transport-Security"); (got != "") != tt.wantHSTS {
				t.Fatalf("HSTS header = %q, want present %t", got, tt.wantHSTS)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
				t.Fatalf("CORS origin = %q, want configured origin", got)
			}
		})
	}
}

func TestIsLocalAppEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "local", env: "local", want: true},
		{name: "development ignores case and spaces", env: " Development ", want: true},
		{name: "test", env: "test", want: true},
		{name: "production", env: "production", want: false},
		{name: "empty", env: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalAppEnv(tt.env); got != tt.want {
				t.Fatalf("isLocalAppEnv(%q) = %t, want %t", tt.env, got, tt.want)
			}
		})
	}
}

func TestBuildIPExtractor(t *testing.T) {
	tests := []struct {
		name         string
		trustedCIDRs []string
		remoteAddr   string
		forwardedFor string
		want         string
	}{
		{
			name:         "direct connection ignores spoofed forwarded header",
			remoteAddr:   "203.0.113.10:1234",
			forwardedFor: "198.51.100.7",
			want:         "203.0.113.10",
		},
		{
			name:         "trusted proxy accepts forwarded client",
			trustedCIDRs: []string{"203.0.113.0/24"},
			remoteAddr:   "203.0.113.10:1234",
			forwardedFor: "198.51.100.7",
			want:         "198.51.100.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor, err := buildIPExtractor(tt.trustedCIDRs)
			if err != nil {
				t.Fatalf("buildIPExtractor() error = %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set(echo.HeaderXForwardedFor, tt.forwardedFor)

			if got := extractor(req); got != tt.want {
				t.Fatalf("extracted IP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildIPExtractorRejectsInvalidCIDR(t *testing.T) {
	_, err := buildIPExtractor([]string{"not-a-cidr"})
	if err == nil {
		t.Fatal("expected invalid CIDR error")
	}
}
