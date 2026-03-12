package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

type stubLogger struct{}

func (stubLogger) Debug(context.Context, string, ...logger.Fields) {}
func (stubLogger) Info(context.Context, string, ...logger.Fields)  {}
func (stubLogger) Warn(context.Context, string, ...logger.Fields)  {}
func (stubLogger) Error(context.Context, string, error, ...logger.Fields) {
}
func (stubLogger) SetLogSink(logger.LogSinkFunc) {}
func (stubLogger) SetContextFieldsProvider(logger.ContextFieldsProviderFunc) {
}
func (stubLogger) Sync() error { return nil }

type stubRegistrar struct{}

func (stubRegistrar) RegisterRoutes(e *echo.Echo) {
	e.GET("/ping", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
}

func TestNewServerRegistersRoutes(t *testing.T) {
	server, err := NewServer(ServerConfig{Address: ":0"}, stubLogger{}, nil, nil, stubRegistrar{})
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

	server, err := NewServer(ServerConfig{Address: ":0"}, stubLogger{}, nil, []echo.MiddlewareFunc{nil, mw, nil}, stubRegistrar{})
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
	server, err := NewServer(ServerConfig{Address: ":0"}, stubLogger{}, nil, nil, nil, stubRegistrar{}, nil)
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

	_, err := NewServer(ServerConfig{Address: ":0"}, stubLogger{}, []ValidationRegistrar{failingRegistrar}, nil)
	if err == nil {
		t.Fatal("expected error from failing validator registrar, got nil")
	}
}
