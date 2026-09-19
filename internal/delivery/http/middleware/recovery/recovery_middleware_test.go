package recovery

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
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

func TestMiddlewareRecoversPanicWithStandardInternalResponse(t *testing.T) {
	log := &loggertest.Logger{ErrorFunc: func(context.Context, string, error, ...logger.Fields) {}}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := New(log, nil).Handler()(func(*echo.Context) error {
		panic("database password")
	})(c)
	if err != nil {
		t.Fatalf("middleware error = %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if body.Code != string(apperr.CodeInternal) || body.Message != "internal server error" {
		t.Fatalf("body = %#v", body)
	}

	outcome, _ := httpcontext.ErrorOutcomeFrom(c)
	if outcome.OriginalError == nil || outcome.OriginalError.Error() != "panic: database password" {
		t.Fatalf("recorded error = %v", outcome.OriginalError)
	}
	if outcome.ApplicationErrorCode != string(apperr.CodeInternal) || outcome.ApplicationErrorMessage != "internal server error" {
		t.Fatalf("error outcome = %#v", outcome)
	}
	assertPanicLog(t, log)
}

func TestMiddlewarePreservesPanicErrorCause(t *testing.T) {
	cause := errors.New("repository panic")
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/panic", nil), httptest.NewRecorder())

	if err := New(nil, nil).Handler()(func(*echo.Context) error {
		panic(cause)
	})(c); err != nil {
		t.Fatalf("middleware error = %v", err)
	}
	outcome, _ := httpcontext.ErrorOutcomeFrom(c)
	if !errors.Is(outcome.OriginalError, cause) {
		t.Fatal("recorded panic does not preserve its error cause")
	}
}

func TestMiddlewareDoesNotOverwriteCommittedResponse(t *testing.T) {
	log := &loggertest.Logger{ErrorFunc: func(context.Context, string, error, ...logger.Fields) {}}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := New(log, nil).Handler()(func(c *echo.Context) error {
		if writeErr := c.String(http.StatusOK, "partial"); writeErr != nil {
			return writeErr
		}
		panic("stream failed")
	})(c)
	if err == nil || err.Error() != "panic: stream failed" {
		t.Fatalf("middleware error = %v, want committed-response panic", err)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != "partial" {
		t.Fatalf("response = (%d, %q), want unchanged partial response", rec.Code, rec.Body.String())
	}
	outcome, _ := httpcontext.ErrorOutcomeFrom(c)
	if outcome.OriginalError == nil || outcome.OriginalError.Error() != "panic: stream failed" {
		t.Fatalf("recorded error = %v", outcome.OriginalError)
	}
	assertPanicLog(t, log)
}

func TestMiddlewarePreservesAbortHandlerPanic(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/abort", nil), httptest.NewRecorder())
	handler := New(nil, nil).Handler()(func(*echo.Context) error {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("recovered value = %v, want http.ErrAbortHandler", recovered)
		}
	}()
	_ = handler(c)
}

func TestMiddlewareReturnsOrdinaryHandlerErrorUnchanged(t *testing.T) {
	handlerErr := errors.New("mapped handler error")
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/error", nil), httptest.NewRecorder())

	err := New(nil, nil).Handler()(func(*echo.Context) error { return handlerErr })(c)
	if err != handlerErr {
		t.Fatalf("error = %v, want unchanged handler error", err)
	}
}

func assertPanicLog(t *testing.T, log *loggertest.Logger) {
	t.Helper()
	if len(log.ErrorCalls) != 1 {
		t.Fatalf("panic error logs = %#v", log.ErrorCalls)
	}
	stack, ok := log.ErrorCalls[0].Fields[0]["panic.stack"].(string)
	if !ok || stack == "" {
		t.Fatal("panic stack was not logged")
	}
}
