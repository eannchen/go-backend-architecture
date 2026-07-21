package bodylimitmw

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestMiddlewareRejectsDeclaredOversizedBody(t *testing.T) {
	const maxBytes = 16
	e := echo.New()
	e.Pre(New(maxBytes).Handler())
	e.POST("/read", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/read", strings.NewReader(strings.Repeat("x", maxBytes+1)))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestMiddlewareBoundsChunkedBody(t *testing.T) {
	const maxBytes = 16
	e := echo.New()
	e.Pre(New(maxBytes).Handler())
	e.POST("/read", func(c *echo.Context) error {
		_, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "request body too large")
		}
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/read", strings.NewReader(strings.Repeat("x", maxBytes+1)))
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
