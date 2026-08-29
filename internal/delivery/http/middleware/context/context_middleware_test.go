package contextmw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestRequestContextMiddlewareTimeoutSkipper(t *testing.T) {
	const streamPath = "/stream"

	tests := []struct {
		name         string
		path         string
		requestID    string
		wantDeadline bool
	}{
		{name: "ordinary request has a deadline and preserves ID", path: "/health", requestID: "request-123", wantDeadline: true},
		{name: "stream request skips the deadline and generates ID", path: streamPath, wantDeadline: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := NewRequestContextMiddleware(5*time.Second, nil, WithTimeoutSkipper(func(c *echo.Context) bool {
				return c.Request().URL.Path == streamPath
			}))

			var gotDeadline bool
			var gotRequestID string
			handler := mw.Handler()(func(c *echo.Context) error {
				_, gotDeadline = c.Request().Context().Deadline()
				gotRequestID = observability.RequestIDFromContext(c.Request().Context())
				return c.NoContent(http.StatusOK)
			})

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.requestID != "" {
				req.Header.Set(requestIDHeader, tt.requestID)
			}
			rec := httptest.NewRecorder()
			if err := handler(e.NewContext(req, rec)); err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if gotDeadline != tt.wantDeadline {
				t.Fatalf("deadline present = %v, want %v", gotDeadline, tt.wantDeadline)
			}
			responseID := rec.Header().Get(requestIDHeader)
			if responseID == "" || responseID != gotRequestID {
				t.Fatalf("response ID = %q, context ID = %q; want matching IDs", responseID, gotRequestID)
			}
			if tt.requestID != "" && responseID != tt.requestID {
				t.Fatalf("response ID = %q, want preserved ID %q", responseID, tt.requestID)
			}
			if !isValidRequestID(responseID) {
				t.Fatalf("generated request ID %q is invalid", responseID)
			}
		})
	}
}

func TestRequestContextMiddlewareRejectsInvalidRequestID(t *testing.T) {
	tests := []string{
		"contains spaces",
		"contains/slash",
		"非ascii",
		strings.Repeat("x", maxRequestIDLen+1),
	}

	for _, requestID := range tests {
		t.Run(requestID, func(t *testing.T) {
			mw := NewRequestContextMiddleware(time.Second, nil)
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(requestIDHeader, requestID)
			rec := httptest.NewRecorder()
			nextCalls := 0

			err := mw.Handler()(func(*echo.Context) error {
				nextCalls++
				return nil
			})(e.NewContext(req, rec))

			if err != nil {
				t.Fatalf("middleware error = %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if nextCalls != 0 {
				t.Fatalf("next calls = %d, want 0", nextCalls)
			}
		})
	}
}
