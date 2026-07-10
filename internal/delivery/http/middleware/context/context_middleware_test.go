package contextmw

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

func TestRequestContextMiddlewareTimeoutSkipper(t *testing.T) {
	const streamPath = "/stream"

	tests := []struct {
		name         string
		path         string
		wantDeadline bool
	}{
		{name: "ordinary request has a deadline", path: "/health", wantDeadline: true},
		{name: "stream request skips the deadline", path: streamPath, wantDeadline: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := NewRequestContextMiddleware(5*time.Second, nil, WithTimeoutSkipper(func(c *echo.Context) bool {
				return c.Request().URL.Path == streamPath
			}))

			var gotDeadline bool
			handler := mw.Handler()(func(c *echo.Context) error {
				_, gotDeadline = c.Request().Context().Deadline()
				return c.NoContent(http.StatusOK)
			})

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			if err := handler(e.NewContext(req, rec)); err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if gotDeadline != tt.wantDeadline {
				t.Fatalf("deadline present = %v, want %v", gotDeadline, tt.wantDeadline)
			}
			if rec.Header().Get(requestIDHeader) == "" {
				t.Fatal("expected request ID header")
			}
		})
	}
}
