package httpcontext

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

// TestErrorOutcomeReadWrite checks an error outcome stored on a request can be recovered by later middleware.
func TestErrorOutcomeReadWrite(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	wantErr := errors.New("boom")
	wantDetails := Details{"stage": "bind"}
	SetErrorOutcome(c, ErrorOutcome{
		OriginalError:           wantErr,
		ApplicationErrorCode:    "INVALID_QUERY",
		ApplicationErrorMessage: "invalid query",
		DiagnosticDetails:       wantDetails,
	})

	outcome, ok := ErrorOutcomeFrom(c)
	if !ok {
		t.Fatal("error outcome was not found")
	}
	if outcome.OriginalError != wantErr {
		t.Fatalf("original error = %v, want %v", outcome.OriginalError, wantErr)
	}
	if outcome.ApplicationErrorCode != "INVALID_QUERY" || outcome.ApplicationErrorMessage != "invalid query" {
		t.Fatalf("application error = %#v", outcome)
	}
	if outcome.DiagnosticDetails["stage"] != "bind" {
		t.Fatalf("diagnostic details = %#v, want bind stage", outcome.DiagnosticDetails)
	}
}

// TestMissingErrorOutcomeReturnsFalse checks a request with no stored outcome is distinguishable from an empty outcome.
func TestMissingErrorOutcomeReturnsFalse(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	if outcome, ok := ErrorOutcomeFrom(c); ok || outcome.OriginalError != nil || outcome.ApplicationErrorCode != "" || outcome.ApplicationErrorMessage != "" || len(outcome.DiagnosticDetails) != 0 {
		t.Fatalf("error outcome = (%#v, %t), want zero value and false", outcome, ok)
	}
}
