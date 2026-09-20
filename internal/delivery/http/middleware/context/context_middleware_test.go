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

// TestRequestContextMiddlewareTimeoutSkipper checks selected routes can bypass request timeout policy.
func TestRequestContextMiddlewareTimeoutSkipper(t *testing.T) {
	const streamPath = "/stream"

	tests := []struct {
		name         string
		path         string
		requestID    string
		wantDeadline bool
	}{
		{name: "ordinary request has a deadline and preserves ID", path: "/health", requestID: "request-123", wantDeadline: true},
		{name: "stream request skips the deadline without inventing ID", path: streamPath, wantDeadline: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := newTestMiddleware(t, Config{
				Timeout: 5 * time.Second,
				RequestID: RequestIDConfig{
					IncomingHeaderKey: "X-Correlation-ID",
					ResponseHeaderKey: "X-Response-ID",
				},
			}, WithTimeoutSkipper(func(c *echo.Context) bool {
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
				req.Header.Set("X-Correlation-ID", tt.requestID)
			}
			rec := httptest.NewRecorder()
			if err := handler(e.NewContext(req, rec)); err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if gotDeadline != tt.wantDeadline {
				t.Fatalf("deadline present = %v, want %v", gotDeadline, tt.wantDeadline)
			}
			responseID := rec.Header().Get("X-Response-ID")
			if tt.requestID == "" {
				if gotRequestID != "" || responseID != "" {
					t.Fatalf("context ID = %q, response ID = %q; want neither", gotRequestID, responseID)
				}
			} else if gotRequestID != tt.requestID || responseID != tt.requestID {
				t.Fatalf("context ID = %q, response ID = %q; want %q", gotRequestID, responseID, tt.requestID)
			}
		})
	}
}

// TestRequestContextMiddlewareHandlesInvalidRequestIDByPolicy checks invalid request IDs follow the configured accept-or-reject policy.
func TestRequestContextMiddlewareHandlesInvalidRequestIDByPolicy(t *testing.T) {
	tests := []string{
		"contains spaces",
		"contains/slash",
		"非ascii",
		strings.Repeat("x", 129),
	}

	for _, requestID := range tests {
		for _, rejectInvalid := range []bool{false, true} {
			policy := "permissive"
			if rejectInvalid {
				policy = "strict"
			}
			t.Run(policy+"/"+requestID, func(t *testing.T) {
				mw := newTestMiddleware(t, Config{RequestID: RequestIDConfig{
					IncomingHeaderKey: "X-Correlation-ID",
					ResponseHeaderKey: "X-Response-ID",
					RejectInvalid:     rejectInvalid,
				}})
				e := echo.New()
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Correlation-ID", requestID)
				rec := httptest.NewRecorder()
				nextCalls := 0

				err := mw.Handler()(func(*echo.Context) error {
					nextCalls++
					return nil
				})(e.NewContext(req, rec))

				if err != nil {
					t.Fatalf("middleware error = %v", err)
				}
				if rejectInvalid {
					if rec.Code != http.StatusBadRequest || nextCalls != 0 {
						t.Fatalf("strict result = status %d, next calls %d", rec.Code, nextCalls)
					}
				} else if rec.Code != http.StatusOK || nextCalls != 1 {
					t.Fatalf("permissive result = status %d, next calls %d", rec.Code, nextCalls)
				}
			})
		}
	}
}

// TestRequestContextMiddlewareEmptyKeysDisableRequestID checks empty header keys disable request ID extraction.
func TestRequestContextMiddlewareEmptyKeysDisableRequestID(t *testing.T) {
	mw := newTestMiddleware(t, Config{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "request-123")
	rec := httptest.NewRecorder()

	err := mw.Handler()(func(c *echo.Context) error {
		if got := observability.RequestIDFromContext(c.Request().Context()); got != "" {
			t.Fatalf("request ID = %q, want disabled", got)
		}
		return c.NoContent(http.StatusOK)
	})(e.NewContext(req, rec))
	if err != nil {
		t.Fatalf("middleware error = %v", err)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "" {
		t.Fatalf("response request ID = %q, want none", got)
	}
}

// TestNewRequestContextMiddlewareRejectsInvalidHeaderKey checks invalid header configuration fails before serving traffic.
func TestNewRequestContextMiddlewareRejectsInvalidHeaderKey(t *testing.T) {
	_, err := NewRequestContextMiddleware(Config{RequestID: RequestIDConfig{IncomingHeaderKey: "request id"}}, nil)
	if err == nil {
		t.Fatal("NewRequestContextMiddleware() error = nil, want invalid header error")
	}
}

func newTestMiddleware(t *testing.T, config Config, opts ...Option) *RequestContextMiddleware {
	t.Helper()
	middleware, err := NewRequestContextMiddleware(config, nil, opts...)
	if err != nil {
		t.Fatalf("NewRequestContextMiddleware() error = %v", err)
	}
	return middleware
}
