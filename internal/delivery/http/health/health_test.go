package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"go-backend-architecture/internal/apperr"
	deliveryhttp "go-backend-architecture/internal/delivery/http"
	openapi "go-backend-architecture/internal/delivery/http/openapi/gen"
	"go-backend-architecture/internal/logger"
	usecasehealth "go-backend-architecture/internal/usecase/health"
)

type stubLogger struct{}

func (stubLogger) Debug(context.Context, string, ...logger.Fields) {}
func (stubLogger) Info(context.Context, string, ...logger.Fields)  {}
func (stubLogger) Warn(context.Context, string, ...logger.Fields)  {}
func (stubLogger) Error(context.Context, string, error, ...logger.Fields) {
}
func (stubLogger) SetLogSink(logger.LogSinkFunc) {}
func (stubLogger) SetContextFieldsProvider(logger.ContextFieldsProviderFunc) {
}
func (stubLogger) Sync() error { return nil }

type stubUsecase struct {
	result   usecasehealth.Result
	err      error
	calls    int
	lastMode usecasehealth.CheckMode
	lastCtx  context.Context
}

func (s *stubUsecase) Check(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
	s.calls++
	s.lastCtx = ctx
	s.lastMode = mode
	return s.result, s.err
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
	h := NewHandler(stubLogger{}, nil, uc)

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
	h := NewHandler(stubLogger{}, nil, uc)

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

	var got deliveryhttp.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != deliveryhttp.ErrCodeInvalidQuery {
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
	h := NewHandler(stubLogger{}, nil, uc)

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
