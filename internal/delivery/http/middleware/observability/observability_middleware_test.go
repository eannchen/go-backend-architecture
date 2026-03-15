package observabilitymw

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
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
