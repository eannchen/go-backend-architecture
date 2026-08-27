package httpcontext

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestMetadataReadWrite(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	wantErr := errors.New("boom")
	wantDetails := Details{"stage": "bind"}
	SetError(c, wantErr)
	SetErrorDetails(c, wantDetails)
	SetTransportError(c, "INVALID_QUERY", "invalid query")

	if got := Error(c); got != wantErr {
		t.Fatalf("error = %v, want %v", got, wantErr)
	}
	if got := ErrorDetails(c); got["stage"] != "bind" {
		t.Fatalf("details = %#v, want bind stage", got)
	}
	if code, message := TransportError(c); code != "INVALID_QUERY" || message != "invalid query" {
		t.Fatalf("transport error = %q %q, want INVALID_QUERY invalid query", code, message)
	}
}

func TestMissingMetadataReturnsZeroValues(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	if Error(c) != nil || ErrorDetails(c) != nil {
		t.Fatal("expected missing error metadata to return nil")
	}
	if code, message := TransportError(c); code != "" || message != "" {
		t.Fatalf("transport error = %q %q, want empty values", code, message)
	}
}
