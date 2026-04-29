package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

type stubRegistrar struct{}

func (stubRegistrar) RegisterRoutes(e *echo.Echo) {
	e.GET("/ping", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
}

func TestNewServerRegistersRoutes(t *testing.T) {
	server, err := NewServer(ServerConfig{Address: ":0"}, &loggertest.Logger{}, nil, nil, nil, stubRegistrar{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

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

	server, err := NewServer(ServerConfig{Address: ":0"}, &loggertest.Logger{}, nil, nil, []echo.MiddlewareFunc{nil, mw, nil}, stubRegistrar{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected non-nil middleware to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestNewServerSkipsNilRegistrar(t *testing.T) {
	server, err := NewServer(ServerConfig{Address: ":0"}, &loggertest.Logger{}, nil, nil, nil, nil, stubRegistrar{}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestNewServerValidationRegistrarFailure(t *testing.T) {
	failingRegistrar := func(v *validator.Validate) error {
		return errors.New("registration failed")
	}

	_, err := NewServer(ServerConfig{Address: ":0"}, &loggertest.Logger{}, nil, []ValidationRegistrar{failingRegistrar}, nil)
	if err == nil {
		t.Fatal("expected error from failing validator registrar, got nil")
	}
}
