package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	httpdeliverytest "github.com/eannchen/go-backend-architecture/internal/delivery/http/httptest"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
	"github.com/eannchen/go-backend-architecture/internal/usecase/health/healthtest"
)

func streamConfig() StreamConfig {
	// Long intervals keep periodic work out of tests that exercise only the
	// immediate event. Each test ends the stream through its request context.
	return StreamConfig{
		CheckInterval:     time.Minute,
		HeartbeatInterval: time.Minute,
		MaxDuration:       2 * time.Minute,
	}
}

func TestStreamHealthWritesInitialHealthEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := usecasehealth.Result{
		Database: usecasehealth.Database{Status: "up", Name: "app"},
		Cache:    usecasehealth.Dependency{Status: "up"},
		KVStore:  usecasehealth.Dependency{Status: "up"},
	}
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			// The handler writes its first event synchronously, so cancellation here
			// lets the stream exit without making this test depend on a timer.
			cancel()
			return result, nil
		},
	}
	h := NewHandler(logger.NoopLogger{}, nil, nil, uc, streamConfig())

	e := echo.New()
	e.Validator = httpdeliverytest.NewValidator(t, RegisterValidation)
	req := httptest.NewRequest(http.MethodGet, StreamPath+"?check=live", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.StreamHealth(c); err != nil {
		t.Fatalf("StreamHealth() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(echo.HeaderContentType); got != "text/event-stream" {
		t.Fatalf("content type = %q, want text/event-stream", got)
	}
	if got := rec.Body.String(); !strings.Contains(got, "event: health\ndata: ") {
		t.Fatalf("stream body = %q, want health event", got)
	}
	if uc.CheckMode != usecasehealth.CheckModeLive {
		t.Fatalf("check mode = %q, want live", uc.CheckMode)
	}
}

func TestStreamHealthRejectsInvalidQueryBeforeOpeningStream(t *testing.T) {
	uc := &healthtest.Usecase{}
	h := NewHandler(logger.NoopLogger{}, nil, nil, uc, streamConfig())
	e := echo.New()
	e.Validator = httpdeliverytest.NewValidator(t, RegisterValidation)
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, StreamPath+"?check=bad", nil), rec)

	if err := h.StreamHealth(c); err != nil {
		t.Fatalf("StreamHealth() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := rec.Header().Get(echo.HeaderContentType); got == "text/event-stream" {
		t.Fatalf("content type = %q, stream must not open for an invalid query", got)
	}
	if uc.CheckCalls != 0 {
		t.Fatalf("Check() calls = %d, want 0", uc.CheckCalls)
	}
}

func TestStreamHealthEmitsDependencyFailureAndLogsWarning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wantErr := apperr.New(apperr.CodeUnavailable, "database unavailable")
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			cancel()
			return usecasehealth.Result{Database: usecasehealth.Database{Status: "down"}}, wantErr
		},
	}
	log := &loggertest.Logger{
		WarnFunc: func(context.Context, string, ...logger.Fields) {},
	}
	h := NewHandler(log, nil, nil, uc, streamConfig())
	e := echo.New()
	e.Validator = httpdeliverytest.NewValidator(t, RegisterValidation)
	req := httptest.NewRequest(http.MethodGet, StreamPath, nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	if err := h.StreamHealth(e.NewContext(req, rec)); err != nil {
		t.Fatalf("StreamHealth() error = %v", err)
	}
	if !strings.Contains(rec.Body.String(), `"status":"down"`) {
		t.Fatalf("stream body = %q, want dependency failure payload", rec.Body.String())
	}
	if len(log.WarnCalls) != 1 || log.WarnCalls[0].Fields[0]["error"] != wantErr {
		t.Fatalf("warning calls = %+v, want dependency error", log.WarnCalls)
	}
}

func TestStreamHealthStopsAfterEventWriteFailure(t *testing.T) {
	wantErr := errors.New("client connection closed")
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			return usecasehealth.Result{Database: usecasehealth.Database{Status: "up"}}, nil
		},
	}
	log := &loggertest.Logger{
		WarnFunc: func(context.Context, string, ...logger.Fields) {},
	}
	h := NewHandler(log, nil, nil, uc, streamConfig())
	e := echo.New()
	e.Validator = httpdeliverytest.NewValidator(t, RegisterValidation)
	w := &failingHealthStreamWriter{header: make(http.Header), err: wantErr}
	c := e.NewContext(httptest.NewRequest(http.MethodGet, StreamPath, nil), w)

	if err := h.StreamHealth(c); err != nil {
		t.Fatalf("StreamHealth() error = %v", err)
	}
	if uc.CheckCalls != 1 {
		t.Fatalf("Check() calls = %d, want 1", uc.CheckCalls)
	}
	if len(log.WarnCalls) != 1 {
		t.Fatalf("warning calls = %d, want 1", len(log.WarnCalls))
	}
	got := log.WarnCalls[0]
	if got.Message != "write health stream event failed" || !errors.Is(got.Fields[0]["error"].(error), wantErr) {
		t.Fatalf("warning call = %+v, want event write failure", got)
	}
}

// failingHealthStreamWriter still supports flushing, allowing the test to
// reach the event-write failure rather than failing during SSE setup.
type failingHealthStreamWriter struct {
	header http.Header
	err    error
	status int
}

func (w *failingHealthStreamWriter) Header() http.Header {
	return w.header
}

func (w *failingHealthStreamWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func (w *failingHealthStreamWriter) WriteHeader(status int) {
	w.status = status
}

func (*failingHealthStreamWriter) Flush() {}
