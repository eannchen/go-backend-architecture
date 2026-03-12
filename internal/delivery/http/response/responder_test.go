package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

func TestResponderAppErrorUsesInternalForNonAppError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	responder := NewResponder(nil)
	if err := responder.AppError(c, errors.New("db down")); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != string(apperr.CodeInternal) {
		t.Fatalf("expected code %q, got %#v", apperr.CodeInternal, body["code"])
	}
	if body["message"] != "internal server error" {
		t.Fatalf("expected internal message, got %#v", body["message"])
	}
}

func TestResponderAppErrorCopiesAppErrorFields(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	details := apperr.Fields("field", "name")
	appErr := apperr.New(apperr.CodeInvalidArgument, "bad input", details)

	responder := NewResponder(nil)
	if err := responder.AppError(c, appErr); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != string(apperr.CodeInvalidArgument) {
		t.Fatalf("expected code %q, got %#v", apperr.CodeInvalidArgument, body["code"])
	}
	if body["message"] != "bad input" {
		t.Fatalf("expected message %q, got %#v", "bad input", body["message"])
	}
	if _, ok := body["details"]; ok {
		t.Fatalf("details should not be exposed in response payload")
	}

	code, msg := ContextMeta{}.GetTransportError(c)
	if code != string(apperr.CodeInvalidArgument) {
		t.Fatalf("expected transport code %q, got %q", apperr.CodeInvalidArgument, code)
	}
	if msg != "bad input" {
		t.Fatalf("expected transport message %q, got %q", "bad input", msg)
	}
}

func TestResponderErrorWritesBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	responder := NewResponder(nil)
	if err := responder.Error(c, errors.New("bad input"), Code("BAD_INPUT"), "bad input"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var body errorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != "BAD_INPUT" || body.Message != "bad input" {
		t.Fatalf("expected error body to match written payload, got %#v", body)
	}

	code, msg := ContextMeta{}.GetTransportError(c)
	if code != "BAD_INPUT" {
		t.Fatalf("expected transport code %q, got %q", "BAD_INPUT", code)
	}
	if msg != "bad input" {
		t.Fatalf("expected transport message %q, got %q", "bad input", msg)
	}
}

func TestResponderInvalidQueryStoresInternalDetailsOnly(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	responder := NewResponder(nil)
	err := errors.New("bind failed")
	if callErr := responder.InvalidQuery(c, err, "invalid query", Fields("field", "check")); callErr != nil {
		t.Fatalf("expected no error, got %v", callErr)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	var body map[string]any
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if _, ok := body["details"]; ok {
		t.Fatalf("details should not be exposed in response payload")
	}

	details := ContextMeta{}.GetErrorDetails(c)
	if details == nil || details["field"] != "check" {
		t.Fatalf("expected internal error details, got %#v", details)
	}
}
