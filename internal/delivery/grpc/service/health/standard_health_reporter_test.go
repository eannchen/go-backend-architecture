package health

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
	"github.com/eannchen/go-backend-architecture/internal/usecase/health/healthtest"
)

type recordingStandardHealthServer struct {
	mu            sync.Mutex
	statuses      map[string][]healthpb.HealthCheckResponse_ServingStatus
	shutdownCalls int
}

func (s *recordingStandardHealthServer) SetServingStatus(service string, status healthpb.HealthCheckResponse_ServingStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.statuses == nil {
		s.statuses = make(map[string][]healthpb.HealthCheckResponse_ServingStatus)
	}
	s.statuses[service] = append(s.statuses[service], status)
}

func (s *recordingStandardHealthServer) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdownCalls++
}

func (s *recordingStandardHealthServer) serviceStatuses(service string) []healthpb.HealthCheckResponse_ServingStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]healthpb.HealthCheckResponse_ServingStatus(nil), s.statuses[service]...)
}

func (s *recordingStandardHealthServer) shutdownCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdownCalls
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
	server := &recordingStandardHealthServer{}
	reporter := newTestReporter(t, time.Hour, uc, server)

	assertStatuses(t, server.serviceStatuses(overallService), healthpb.HealthCheckResponse_NOT_SERVING)
	assertStatuses(t, server.serviceStatuses("diagnostics.v1.DiagnosticsService"), healthpb.HealthCheckResponse_NOT_SERVING)

	if err := reporter.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	assertStatuses(t, server.serviceStatuses(overallService), healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING)
	assertStatuses(t, server.serviceStatuses("diagnostics.v1.DiagnosticsService"), healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING)

	readinessErr = errors.New("cache down")
	if err := reporter.Refresh(context.Background()); !errors.Is(err, readinessErr) {
		t.Fatalf("Refresh() error = %v, want %v", err, readinessErr)
	}
	assertStatuses(t, server.serviceStatuses(overallService), healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING, healthpb.HealthCheckResponse_NOT_SERVING)
	assertStatuses(t, server.serviceStatuses("diagnostics.v1.DiagnosticsService"), healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVING, healthpb.HealthCheckResponse_NOT_SERVING)
}

func TestReporterStartsImmediatelyAndRefreshesPeriodically(t *testing.T) {
	checkCalls := make(chan struct{}, 2)
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			select {
			case checkCalls <- struct{}{}:
			default:
			}
			return usecasehealth.Result{}, nil
		},
	}
	server := &recordingStandardHealthServer{}
	reporter := newTestReporter(t, 5*time.Millisecond, uc, server)

	reporter.Start()
	for call := 0; call < 2; call++ {
		select {
		case <-checkCalls:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for readiness check %d", call+1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := reporter.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if server.shutdownCount() != 1 {
		t.Fatalf("standard health shutdown calls = %d, want 1", server.shutdownCount())
	}
}

func TestReporterShutdownHonorsContextWhileRefreshIsBlocked(t *testing.T) {
	checkStarted := make(chan struct{})
	releaseCheck := make(chan struct{})
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			close(checkStarted)
			<-releaseCheck
			return usecasehealth.Result{}, nil
		},
	}
	server := &recordingStandardHealthServer{}
	reporter := newTestReporter(t, time.Hour, uc, server)
	reporter.Start()
	<-checkStarted

	shutdownCtx, cancelShutdown := context.WithCancel(context.Background())
	cancelShutdown()
	if err := reporter.Shutdown(shutdownCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown() error = %v, want context cancellation", err)
	}

	close(releaseCheck)
	waitCtx, cancelWait := context.WithTimeout(context.Background(), time.Second)
	defer cancelWait()
	if err := reporter.Shutdown(waitCtx); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
	if got := server.shutdownCount(); got != 1 {
		t.Fatalf("standard health shutdown calls = %d, want 1", got)
	}
}

func TestNewReporterRejectsInvalidRefreshInterval(t *testing.T) {
	_, err := NewReporter(
		ReporterConfig{},
		nil,
		&healthtest.Usecase{},
		&recordingStandardHealthServer{},
		"",
	)
	if err == nil {
		t.Fatal("NewReporter() error = nil, want invalid interval error")
	}
}

func newTestReporter(
	t *testing.T,
	interval time.Duration,
	uc usecasehealth.Usecase,
	server StandardHealthServer,
) *Reporter {
	t.Helper()
	reporter, err := NewReporter(
		ReporterConfig{RefreshInterval: interval},
		logger.NoopLogger{},
		uc,
		server,
		"diagnostics.v1.DiagnosticsService",
	)
	if err != nil {
		t.Fatalf("NewReporter() error = %v", err)
	}
	return reporter
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
