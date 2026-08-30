package httpcontext

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestContextMetaReadWrite(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	wantErr := errors.New("boom")
	wantDetails := Details{"stage": "bind"}
	meta := ContextMeta{}

	meta.SetError(c, wantErr)
	meta.SetErrorDetails(c, wantDetails)
	meta.SetTransportError(c, "INVALID_QUERY", "invalid query")

	if got := meta.GetError(c); got != wantErr {
		t.Fatalf("error = %v, want %v", got, wantErr)
	}
	if got := meta.GetErrorDetails(c); got["stage"] != "bind" {
		t.Fatalf("details = %#v, want bind stage", got)
	}
	if code, message := meta.GetTransportError(c); code != "INVALID_QUERY" || message != "invalid query" {
		t.Fatalf("transport error = %q %q, want INVALID_QUERY invalid query", code, message)
	}
}

func TestContextMetaMissingValuesReturnZeroValues(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	meta := ContextMeta{}

	if meta.GetError(c) != nil || meta.GetErrorDetails(c) != nil {
		t.Fatal("expected missing error metadata to return nil")
	}
	if code, message := meta.GetTransportError(c); code != "" || message != "" {
		t.Fatalf("transport error = %q %q, want empty values", code, message)
	}
}
