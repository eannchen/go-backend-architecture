package observabilitymw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type metricSample struct {
	value  int64
	fields observability.Fields
}

type recordingMeter struct {
	counters   map[string][]metricSample
	histograms map[string]int
}

func newRecordingMeter() *recordingMeter {
	return &recordingMeter{counters: make(map[string][]metricSample), histograms: make(map[string]int)}
}

func (m *recordingMeter) Counter(name string, _ ...observability.MetricOption) observability.Counter {
	return recordingCounter{m: m, name: name}
}

func (m *recordingMeter) UpDownCounter(string, ...observability.MetricOption) observability.UpDownCounter {
	return observability.NoopMeter{}.UpDownCounter("")
}

func (m *recordingMeter) Histogram(name string, _ ...observability.MetricOption) observability.Histogram {
	return recordingHistogram{m: m, name: name}
}

type recordingCounter struct {
	m    *recordingMeter
	name string
}

func (c recordingCounter) Add(_ context.Context, value int64, fields ...observability.Fields) {
	c.m.counters[c.name] = append(c.m.counters[c.name], metricSample{value: value, fields: observability.OptionalFields(fields...)})
}

type recordingHistogram struct {
	m    *recordingMeter
	name string
}

func (h recordingHistogram) Record(context.Context, float64, ...observability.Fields) {
	h.m.histograms[h.name]++
}

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
	meter := newRecordingMeter()
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
	requests := meter.counters["http_server_requests_total"]
	if len(requests) != 1 || requests[0].fields["http.route"] != "/protected" || requests[0].fields["http.response.status_code"] != "401" {
		t.Fatalf("request metric = %#v", requests)
	}
	if errors := meter.counters["http_server_errors_total"]; len(errors) != 1 {
		t.Fatalf("error metric count = %d, want 1", len(errors))
	}
	if got := meter.histograms["http_server_request_duration_seconds"]; got != 1 {
		t.Fatalf("latency metric count = %d, want 1", got)
	}
}
