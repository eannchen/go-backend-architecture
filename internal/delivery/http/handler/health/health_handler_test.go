package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type stubUsecase struct {
	result    usecasehealth.Result
	err       error
	checkFunc func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error)
	calls     int
	lastMode  usecasehealth.CheckMode
	lastCtx   context.Context
}

func (s *stubUsecase) Check(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
	s.calls++
	s.lastCtx = ctx
	s.lastMode = mode
	if s.checkFunc != nil {
		return s.checkFunc(ctx, mode)
	}
	return s.result, s.err
}

func streamConfig() StreamConfig {
	return StreamConfig{
		CheckInterval:     time.Minute,
		HeartbeatInterval: time.Minute,
		MaxDuration:       2 * time.Minute,
	}
}

type echoValidator struct {
	v *validator.Validate
}

func newEchoValidator(t *testing.T) *echoValidator {
	t.Helper()
	v := validator.New()
	if err := RegisterValidation(v); err != nil {
		t.Fatalf("register validation: %v", err)
	}
	return &echoValidator{v: v}
}

func (v *echoValidator) Validate(i any) error {
	return v.v.Struct(i)
}

func TestGetHealthSuccess(t *testing.T) {
	uc := &stubUsecase{
		result: usecasehealth.Result{
			Database: usecasehealth.Database{
				Status:        "up",
				Name:          "app",
				InRecovery:    false,
				UptimeSeconds: 99,
			},
			Cache:   usecasehealth.Dependency{Status: "up"},
			KVStore: usecasehealth.Dependency{Status: "up"},
		},
	}
	h := NewHandler(&loggertest.Logger{}, nil, nil, uc, streamConfig())

	e := echo.New()
	e.Validator = newEchoValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetHealth(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if uc.lastMode != usecasehealth.CheckModeReady {
		t.Fatalf("expected default ready mode, got %q", uc.lastMode)
	}

	var got openapi.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Database.Name != "app" || got.Database.Status != "up" || got.Kvstore.Status != "up" {
		t.Fatalf("unexpected response payload: %+v", got)
	}
}

func TestGetHealthInvalidQuery(t *testing.T) {
	uc := &stubUsecase{}
	h := NewHandler(&loggertest.Logger{}, nil, nil, uc, streamConfig())

	e := echo.New()
	e.Validator = newEchoValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/health?check=bad", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetHealth(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if uc.calls != 0 {
		t.Fatalf("expected usecase not called for invalid query, got %d", uc.calls)
	}

	var got struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != "INVALID_QUERY" {
		t.Fatalf("unexpected error code: %q", got.Code)
	}
}

func TestGetHealthUnavailableReturnsPartialResult(t *testing.T) {
	uc := &stubUsecase{
		result: usecasehealth.Result{
			Database: usecasehealth.Database{Status: "down"},
			Cache:    usecasehealth.Dependency{Status: "skipped"},
			KVStore:  usecasehealth.Dependency{Status: "skipped"},
		},
		err: apperr.New(apperr.CodeUnavailable, "database readiness failed"),
	}
	h := NewHandler(&loggertest.Logger{}, nil, nil, uc, streamConfig())

	e := echo.New()
	e.Validator = newEchoValidator(t)
	req := httptest.NewRequest(http.MethodGet, "/health?check=ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetHealth(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var got openapi.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Database.Status != "down" || got.Cache.Status != "skipped" || got.Kvstore.Status != "skipped" {
		t.Fatalf("unexpected partial result payload: %+v", got)
	}
}

func TestStreamHealthWritesInitialHealthEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	uc := &stubUsecase{
		result: usecasehealth.Result{
			Database: usecasehealth.Database{Status: "up", Name: "app"},
			Cache:    usecasehealth.Dependency{Status: "up"},
			KVStore:  usecasehealth.Dependency{Status: "up"},
		},
	}
	uc.checkFunc = func(_ context.Context, _ usecasehealth.CheckMode) (usecasehealth.Result, error) {
		cancel()
		return uc.result, nil
	}
	h := NewHandler(&loggertest.Logger{}, nil, nil, uc, streamConfig())

	e := echo.New()
	e.Validator = newEchoValidator(t)
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
	if uc.lastMode != usecasehealth.CheckModeLive {
		t.Fatalf("check mode = %q, want live", uc.lastMode)
	}
}
