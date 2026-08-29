package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
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

	code, msg := httpcontext.ContextMeta{}.GetTransportError(c)
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

	code, msg := httpcontext.ContextMeta{}.GetTransportError(c)
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

	details := httpcontext.ContextMeta{}.GetErrorDetails(c)
	if details == nil || details["field"] != "check" {
		t.Fatalf("expected internal error details, got %#v", details)
	}
}

func TestCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		code Code
		want int
	}{
		{code: CodeInvalidQuery, want: http.StatusBadRequest},
		{code: CodeInvalidRequestID, want: http.StatusBadRequest},
		{code: Code(apperr.CodeInvalidArgument), want: http.StatusBadRequest},
		{code: Code(apperr.CodeUnauthorized), want: http.StatusUnauthorized},
		{code: Code(apperr.CodeForbidden), want: http.StatusForbidden},
		{code: Code(apperr.CodeNotFound), want: http.StatusNotFound},
		{code: Code(apperr.CodeConflict), want: http.StatusConflict},
		{code: Code(apperr.CodeTooManyRequests), want: http.StatusTooManyRequests},
		{code: Code(apperr.CodeUnavailable), want: http.StatusServiceUnavailable},
		{code: Code(apperr.CodeTimeout), want: http.StatusGatewayTimeout},
		{code: Code(apperr.CodeInternal), want: http.StatusInternalServerError},
		{code: Code("UNKNOWN"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.toHTTPStatus(); got != tt.want {
				t.Fatalf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResponderAppErrorWithPayload_UsesErrorStatusAndMetadata(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "application error",
			err:         apperr.New(apperr.CodeUnavailable, "database unavailable", apperr.Fields("dependency", "postgres")),
			wantStatus:  http.StatusServiceUnavailable,
			wantCode:    string(apperr.CodeUnavailable),
			wantMessage: "database unavailable",
		},
		{
			name:        "unknown error",
			err:         errors.New("database failed"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    string(apperr.CodeInternal),
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/health", nil), rec)
			payload := map[string]string{"status": "down"}

			if err := NewResponder(nil).AppErrorWithPayload(c, tt.err, payload); err != nil {
				t.Fatalf("write payload response: %v", err)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if body["status"] != "down" {
				t.Fatalf("payload = %#v, want status down", body)
			}
			code, message := httpcontext.ContextMeta{}.GetTransportError(c)
			if code != tt.wantCode || message != tt.wantMessage {
				t.Fatalf("transport error = %q %q, want %q %q", code, message, tt.wantCode, tt.wantMessage)
			}
		})
	}
}
