package observabilitymw

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
)

func TestContextMetaReadWrite(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	originalErr := errors.New("boom")
	meta := httpcontext.NewContextMeta()
	meta.SetError(c, originalErr)
	meta.SetErrorDetails(c, httpcontext.Details{"stage": "bind"})

	if got := meta.GetError(c); got != originalErr {
		t.Fatalf("unexpected original error: %v", got)
	}
	if got := meta.GetErrorDetails(c); got == nil || got["stage"] != "bind" {
		t.Fatalf("unexpected error details: %#v", got)
	}
}

func TestErrorCauseChain(t *testing.T) {
	root := errors.New("root")
	wrapped := fmt.Errorf("wrapped: %w", root)
	got := errorCauseChain(wrapped)
	if got == "" {
		t.Fatalf("expected non-empty cause chain")
	}
	if got != "wrapped: root; root" {
		t.Fatalf("unexpected cause chain: %q", got)
	}
}

func TestAccessLogMiddlewareAcceptsNilLogger(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := NewAccessLogMiddleware(nil, nil).Handler()(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	if err := handler(c); err != nil {
		t.Fatalf("handler() error = %v", err)
	}
}

func TestRequestMetricsMiddlewareRecordsBoundedRouteAndError(t *testing.T) {
	meter := observabilitytest.NewRecordingMeter()
	e := echo.New()
	e.GET("/protected", NewRequestMetricsMiddleware(meter).Handler()(func(c *echo.Context) error {
		return c.NoContent(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	requests := meter.CounterSamples("http_server_requests_total")
	if len(requests) != 1 || requests[0].Fields["http.route"] != "/protected" || requests[0].Fields["http.response.status_code"] != "401" {
		t.Fatalf("request metric = %#v", requests)
	}
	if errors := meter.CounterSamples("http_server_errors_total"); len(errors) != 1 {
		t.Fatalf("error metric count = %d, want 1", len(errors))
	}
	if samples := meter.HistogramSamples("http_server_request_duration_seconds"); len(samples) != 1 {
		t.Fatalf("latency metric count = %d, want 1", len(samples))
	}
}
