package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"go-backend-architecture/internal/logger"
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

func TestNewServerWithNilTracer(t *testing.T) {
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
