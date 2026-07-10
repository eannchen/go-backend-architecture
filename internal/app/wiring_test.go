package app

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestBuildIPExtractorRejectsInvalidCIDR(t *testing.T) {
	_, err := buildIPExtractor([]string{"not-a-cidr"})
	if err == nil {
		t.Fatal("expected invalid CIDR error")
	}
}

func TestBuildServerAppliesHTTPProtection(t *testing.T) {
	wiring := newWiring(config.Config{
		AppEnv: "production",
		HTTP: config.HTTPConfig{
			Address:          ":0",
			ReadTimeout:      time.Second,
			WriteTimeout:     time.Second,
			IdleTimeout:      time.Second,
			RequestTimeout:   time.Second,
			CORSAllowOrigins: []string{"https://app.example.com"},
		},
	}, &loggertest.Logger{}, observability.NoopTracer{}, observability.NoopMeter{})

	server, err := wiring.buildServer(httpresponse.NewResponder(httpcontext.NewContextMeta()), appHandlers{}, appUsecases{})
	if err != nil {
		t.Fatalf("buildServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.TLS = &tls.ConnectionState{}
	req.Header.Set(echo.HeaderOrigin, "https://app.example.com")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("expected HSTS header outside local environments")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("CORS origin = %q, want configured origin", got)
	}
}
