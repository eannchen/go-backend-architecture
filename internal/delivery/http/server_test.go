package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	httpdeliverytest "github.com/eannchen/go-backend-architecture/internal/delivery/http/httptest"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

func newRouteRegistrar() *httpdeliverytest.RouteRegistrar {
	return &httpdeliverytest.RouteRegistrar{
		RegisterRoutesFunc: func(e *echo.Echo) {
			e.GET("/ping", func(c *echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			})
		},
	}
}

func TestNewServerRegistersRoutes(t *testing.T) {
	server, err := NewServer(ServerConfig{Address: ":0"}, logger.NoopLogger{}, nil, nil, nil, nil, newRouteRegistrar())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestNewServerSkipsNilMiddleware(t *testing.T) {
	called := false
	mw := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			called = true
			return next(c)
		}
	}

	server, err := NewServer(ServerConfig{Address: ":0"}, logger.NoopLogger{}, nil, nil, nil, []echo.MiddlewareFunc{nil, mw, nil}, newRouteRegistrar())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected non-nil middleware to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestNewServerSkipsNilRegistrar(t *testing.T) {
	server, err := NewServer(ServerConfig{Address: ":0"}, logger.NoopLogger{}, nil, nil, nil, nil, newRouteRegistrar(), nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestNewServerValidationRegistrarFailure(t *testing.T) {
	failingRegistrar := func(v *validator.Validate) error {
		return errors.New("registration failed")
	}

	_, err := NewServer(ServerConfig{Address: ":0"}, logger.NoopLogger{}, nil, []ValidationRegistrar{failingRegistrar}, nil, nil)
	if err == nil {
		t.Fatal("expected error from failing validator registrar, got nil")
	}
}

func TestNewServerAppliesPreMiddleware(t *testing.T) {
	preMiddleware := func(echo.HandlerFunc) echo.HandlerFunc {
		return func(*echo.Context) error {
			return echo.NewHTTPError(http.StatusForbidden, "blocked")
		}
	}
	server, err := NewServer(ServerConfig{Address: ":0"}, logger.NoopLogger{}, nil, nil, []echo.MiddlewareFunc{nil, preMiddleware}, nil, newRouteRegistrar())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestNewServerUsesSafeSizeDefaults(t *testing.T) {
	server, err := NewServer(ServerConfig{Address: ":0"}, logger.NoopLogger{}, nil, nil, nil, nil, newRouteRegistrar())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if server.httpServer.MaxHeaderBytes != defaultMaxHeaderBytes {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.httpServer.MaxHeaderBytes, defaultMaxHeaderBytes)
	}
}
