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

	outcome, _ := httpcontext.ErrorOutcomeFrom(c)
	if outcome.ApplicationErrorCode != string(apperr.CodeInvalidArgument) {
		t.Fatalf("expected application error code %q, got %q", apperr.CodeInvalidArgument, outcome.ApplicationErrorCode)
	}
	if outcome.ApplicationErrorMessage != "bad input" {
		t.Fatalf("expected application error message %q, got %q", "bad input", outcome.ApplicationErrorMessage)
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
			outcome, _ := httpcontext.ErrorOutcomeFrom(c)
			if outcome.ApplicationErrorCode != tt.wantCode || outcome.ApplicationErrorMessage != tt.wantMessage {
				t.Fatalf("error outcome = %#v, want code %q and message %q", outcome, tt.wantCode, tt.wantMessage)
			}
			if !errors.Is(outcome.OriginalError, tt.err) {
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

	outcome, _ := httpcontext.ErrorOutcomeFrom(c)
	if outcome.ApplicationErrorCode != "BAD_INPUT" {
		t.Fatalf("expected application error code %q, got %q", "BAD_INPUT", outcome.ApplicationErrorCode)
	}
	if outcome.ApplicationErrorMessage != "bad input" {
		t.Fatalf("expected application error message %q, got %q", "bad input", outcome.ApplicationErrorMessage)
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

	outcome, _ := httpcontext.ErrorOutcomeFrom(c)
	details := outcome.DiagnosticDetails
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

			if err := NewResponder().AppErrorWithPayload(c, tt.err, payload); err != nil {
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
			outcome, _ := httpcontext.ErrorOutcomeFrom(c)
			if outcome.ApplicationErrorCode != tt.wantCode || outcome.ApplicationErrorMessage != tt.wantMessage {
				t.Fatalf("error outcome = %#v, want code %q and message %q", outcome, tt.wantCode, tt.wantMessage)
			}
		})
	}
}
