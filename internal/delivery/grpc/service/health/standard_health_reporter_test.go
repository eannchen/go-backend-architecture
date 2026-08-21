package health

import (
	"context"
	"errors"
	"testing"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
	"github.com/eannchen/go-backend-architecture/internal/usecase/health/healthtest"
)

type recordingStatusSetter struct {
	statuses map[string][]healthpb.HealthCheckResponse_ServingStatus
}

func (s *recordingStatusSetter) SetServingStatus(service string, status healthpb.HealthCheckResponse_ServingStatus) {
	if s.statuses == nil {
		s.statuses = make(map[string][]healthpb.HealthCheckResponse_ServingStatus)
	}
	s.statuses[service] = append(s.statuses[service], status)
}

func TestReporterRefreshesOverallAndServiceStatus(t *testing.T) {
	readinessErr := error(nil)
	uc := &healthtest.Usecase{
		CheckFunc: func(_ context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
			if mode != usecasehealth.CheckModeReady {
				t.Fatalf("mode = %q, want %q", mode, usecasehealth.CheckModeReady)
			}
			return usecasehealth.Result{}, readinessErr
		},
	}
	setter := &recordingStatusSetter{}
	reporter := NewReporter(uc, setter, "diagnostics.v1.DiagnosticsService")

	assertStatuses(t, setter.statuses[overallService], healthpb.HealthCheckResponse_NOT_SERVING)
	assertStatuses(t, setter.statuses["diagnostics.v1.DiagnosticsService"], healthpb.HealthCheckResponse_NOT_SERVING)

	if err := reporter.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	assertStatuses(t, setter.statuses[overallService], healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING)
	assertStatuses(t, setter.statuses["diagnostics.v1.DiagnosticsService"], healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING)

	readinessErr = errors.New("cache down")
	if err := reporter.Refresh(context.Background()); !errors.Is(err, readinessErr) {
		t.Fatalf("Refresh() error = %v, want %v", err, readinessErr)
	}
	assertStatuses(t, setter.statuses[overallService], healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING, healthpb.HealthCheckResponse_NOT_SERVING)
	assertStatuses(t, setter.statuses["diagnostics.v1.DiagnosticsService"], healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING, healthpb.HealthCheckResponse_NOT_SERVING)
}

func assertStatuses(t *testing.T, got []healthpb.HealthCheckResponse_ServingStatus, want ...healthpb.HealthCheckResponse_ServingStatus) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("status count = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("status[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
