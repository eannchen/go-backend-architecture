package response

import (
	"context"
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

	responder := NewResponder()
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

	responder := NewResponder()
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

	code, msg := httpcontext.TransportError(c)
	if code != string(apperr.CodeInvalidArgument) {
		t.Fatalf("expected transport code %q, got %q", apperr.CodeInvalidArgument, code)
	}
	if msg != "bad input" {
		t.Fatalf("expected transport message %q, got %q", "bad input", msg)
	}
}

func TestResponderAppErrorPrioritizesContextErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
		wantBody    bool
	}{
		{
			name:        "deadline overrides application code",
			err:         apperr.Wrap(context.DeadlineExceeded, apperr.CodeUnavailable, "database unavailable"),
			wantStatus:  http.StatusGatewayTimeout,
			wantCode:    string(apperr.CodeTimeout),
			wantMessage: "request timed out",
			wantBody:    true,
		},
		{
			name:        "cancellation is recorded as client closed request",
			err:         apperr.Wrap(context.Canceled, apperr.CodeInternal, "operation failed"),
			wantStatus:  statusClientClosedRequest,
			wantCode:    string(CodeRequestCanceled),
			wantMessage: "request canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := NewResponder().AppError(c, tt.err); err != nil {
				t.Fatalf("AppError() error = %v", err)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			code, message := httpcontext.TransportError(c)
			if code != tt.wantCode || message != tt.wantMessage {
				t.Fatalf("transport error = (%q, %q), want (%q, %q)", code, message, tt.wantCode, tt.wantMessage)
			}
			if !errors.Is(httpcontext.Error(c), tt.err) {
				t.Fatal("original error was not retained for observability")
			}

			if tt.wantBody {
				var body errorPayload
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if body.Code != tt.wantCode || body.Message != tt.wantMessage {
					t.Fatalf("body = %#v", body)
				}
			} else if rec.Body.Len() != 0 {
				t.Fatalf("canceled response body = %q, want empty", rec.Body.String())
			}
		})
	}
}

func TestResponderAppErrorWithPayloadDoesNotReturnPayloadAfterDeadline(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := apperr.Wrap(context.DeadlineExceeded, apperr.CodeUnavailable, "database unavailable")

	if responseErr := NewResponder().AppErrorWithPayload(c, err, map[string]any{"healthy": false}); responseErr != nil {
		t.Fatalf("AppErrorWithPayload() error = %v", responseErr)
	}
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
	var body errorPayload
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Code != string(apperr.CodeTimeout) || body.Message != "request timed out" {
		t.Fatalf("body = %#v", body)
	}
}

func TestResponderErrorWritesBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	responder := NewResponder()
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

	code, msg := httpcontext.TransportError(c)
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

	responder := NewResponder()
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

	details := httpcontext.ErrorDetails(c)
	if details == nil || details["field"] != "check" {
		t.Fatalf("expected internal error details, got %#v", details)
	}
}
