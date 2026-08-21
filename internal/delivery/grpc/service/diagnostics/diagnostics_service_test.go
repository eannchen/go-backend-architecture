package diagnostics

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
	"github.com/eannchen/go-backend-architecture/internal/usecase/health/healthtest"
)

func TestGetHealthMapsReadyResult(t *testing.T) {
	uc := &healthtest.Usecase{
		CheckFunc: func(_ context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
			if mode != usecasehealth.CheckModeReady {
				t.Fatalf("mode = %q, want %q", mode, usecasehealth.CheckModeReady)
			}
			return readyResult(), nil
		},
	}

	got, err := NewService(uc, nil).GetHealth(context.Background(), &diagnosticsv1.GetHealthRequest{})
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	if !got.GetHealthy() {
		t.Fatal("healthy = false, want true")
	}
	if got.GetDatabase().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_UP ||
		got.GetCache().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_UP ||
		got.GetKvStore().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_UP ||
		got.GetVectorStore().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_UP {
		t.Fatalf("unexpected dependency statuses: %v", got)
	}
	if got.GetDatabase().Name == nil || got.GetDatabase().GetName() != "app" {
		t.Fatalf("database name = %#v, want app", got.GetDatabase().Name)
	}
	if got.GetDatabase().InRecovery == nil || got.GetDatabase().GetInRecovery() {
		t.Fatalf("database in_recovery = %#v, want present false", got.GetDatabase().InRecovery)
	}
	if got.GetDatabase().UptimeSeconds == nil || got.GetDatabase().GetUptimeSeconds() != 123 {
		t.Fatalf("database uptime = %#v, want 123", got.GetDatabase().UptimeSeconds)
	}
}

func TestGetHealthMapsLiveMode(t *testing.T) {
	uc := &healthtest.Usecase{
		CheckFunc: func(_ context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
			if mode != usecasehealth.CheckModeLive {
				t.Fatalf("mode = %q, want %q", mode, usecasehealth.CheckModeLive)
			}
			return skippedResult(), nil
		},
	}

	got, err := NewService(uc, nil).GetHealth(context.Background(), &diagnosticsv1.GetHealthRequest{
		Mode: diagnosticsv1.HealthCheckMode_HEALTH_CHECK_MODE_LIVE,
	})
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	if !got.GetHealthy() || got.GetDatabase().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_SKIPPED {
		t.Fatalf("unexpected live response: %v", got)
	}
	if got.GetDatabase().Name != nil || got.GetDatabase().InRecovery != nil || got.GetDatabase().UptimeSeconds != nil {
		t.Fatalf("skipped database details should be absent: %v", got.GetDatabase())
	}
}

func TestGetHealthReturnsUnavailableAsDetailedResult(t *testing.T) {
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			result := readyResult()
			result.Cache.Status = "down"
			result.KVStore.Status = "skipped"
			return result, apperr.New(apperr.CodeUnavailable, "cache readiness failed")
		},
	}

	got, err := NewService(uc, nil).GetHealth(context.Background(), &diagnosticsv1.GetHealthRequest{})
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	if got.GetHealthy() {
		t.Fatal("healthy = true, want false")
	}
	if got.GetCache().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_DOWN ||
		got.GetKvStore().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_SKIPPED {
		t.Fatalf("unexpected partial result: %v", got)
	}
}

func TestGetHealthReturnsWrappedContextErrorsAsStatuses(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		want  codes.Code
	}{
		{name: "canceled", cause: context.Canceled, want: codes.Canceled},
		{name: "deadline exceeded", cause: context.DeadlineExceeded, want: codes.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &healthtest.Usecase{
				CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
					return usecasehealth.Result{}, apperr.Wrap(
						tt.cause,
						apperr.CodeUnavailable,
						"database readiness failed",
					)
				},
			}

			_, err := NewService(uc, nil).GetHealth(context.Background(), &diagnosticsv1.GetHealthRequest{})

			if status.Code(err) != tt.want {
				t.Fatalf("status code = %v, want %v", status.Code(err), tt.want)
			}
			if !errors.Is(err, tt.cause) {
				t.Fatal("response error does not preserve the context cause")
			}
		})
	}
}

func TestGetHealthDoesNotMaskReturnedErrorWhenContextIsCanceled(t *testing.T) {
	returnedErr := errors.New("database failed")
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			return usecasehealth.Result{}, returnedErr
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewService(uc, nil).GetHealth(ctx, &diagnosticsv1.GetHealthRequest{})

	if status.Code(err) != codes.Internal {
		t.Fatalf("status code = %v, want %v", status.Code(err), codes.Internal)
	}
	if !errors.Is(err, returnedErr) {
		t.Fatal("response error does not preserve the returned error")
	}
}

func TestGetHealthRejectsUnknownMode(t *testing.T) {
	uc := &healthtest.Usecase{}

	_, err := NewService(uc, nil).GetHealth(context.Background(), &diagnosticsv1.GetHealthRequest{
		Mode: diagnosticsv1.HealthCheckMode(99),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
	if uc.CheckCalls != 0 {
		t.Fatalf("usecase calls = %d, want 0", uc.CheckCalls)
	}
}

func TestGetHealthHidesUnexpectedErrors(t *testing.T) {
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			return usecasehealth.Result{}, errors.New("database password leaked")
		},
	}

	_, err := NewService(uc, nil).GetHealth(context.Background(), &diagnosticsv1.GetHealthRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("status code = %v, want %v", status.Code(err), codes.Internal)
	}
	if got := status.Convert(err).Message(); got != "internal server error" {
		t.Fatalf("status message = %q, want generic message", got)
	}
}

func readyResult() usecasehealth.Result {
	return usecasehealth.Result{
		Database: usecasehealth.Database{Status: "up", Name: "app", InRecovery: false, UptimeSeconds: 123},
		Cache:    usecasehealth.Dependency{Status: "up"},
		KVStore:  usecasehealth.Dependency{Status: "up"},
		Vector:   usecasehealth.Dependency{Status: "up"},
	}
}

func skippedResult() usecasehealth.Result {
	return usecasehealth.Result{
		Database: usecasehealth.Database{Status: "skipped"},
		Cache:    usecasehealth.Dependency{Status: "skipped"},
		KVStore:  usecasehealth.Dependency{Status: "skipped"},
		Vector:   usecasehealth.Dependency{Status: "skipped"},
	}
}
