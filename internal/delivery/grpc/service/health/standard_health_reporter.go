package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

const overallService = ""

// ReporterConfig controls periodic standard-health updates.
type ReporterConfig struct {
	RefreshInterval time.Duration
}

// StandardHealthServer is the part of the standard gRPC health server managed by Reporter.
type StandardHealthServer interface {
	SetServingStatus(string, healthpb.HealthCheckResponse_ServingStatus)
	Shutdown()
}

// Reporter translates readiness results into cached standard gRPC health states.
type Reporter struct {
	log      logger.Logger
	health   usecasehealth.Usecase
	server   StandardHealthServer
	services []string
	interval time.Duration

	runContext context.Context
	cancelRun  context.CancelFunc
	startRun   sync.Once
	stopServer sync.Once
	runStarted chan struct{}
	runDone    chan struct{}
}

// NewReporter creates a lifecycle-managed standard gRPC health reporter.
func NewReporter(
	cfg ReporterConfig,
	log logger.Logger,
	health usecasehealth.Usecase,
	server StandardHealthServer,
	serviceName string,
) (*Reporter, error) {
	if cfg.RefreshInterval <= 0 {
		return nil, fmt.Errorf("standard gRPC health refresh interval must be positive")
	}
	if log == nil {
		log = logger.NoopLogger{}
	}

	services := []string{overallService}
	server.SetServingStatus(overallService, healthpb.HealthCheckResponse_NOT_SERVING)

	if serviceName != overallService {
		services = append(services, serviceName)
		server.SetServingStatus(serviceName, healthpb.HealthCheckResponse_NOT_SERVING)
	}

	runContext, cancelRun := context.WithCancel(context.Background())
	return &Reporter{
		log:        log,
		health:     health,
		server:     server,
		services:   services,
		interval:   cfg.RefreshInterval,
		runContext: runContext,
		cancelRun:  cancelRun,
		runStarted: make(chan struct{}),
		runDone:    make(chan struct{}),
	}, nil
}

// Start begins periodic readiness reporting.
func (r *Reporter) Start() {
	r.startRun.Do(func() {
		close(r.runStarted)
		go r.run()
	})
}

func (r *Reporter) run() {
	defer close(r.runDone)

	if r.runContext.Err() != nil {
		return
	}
	r.refreshAndLog()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.runContext.Done():
			return
		case <-ticker.C:
			r.refreshAndLog()
		}
	}
}

func (r *Reporter) refreshAndLog() {
	if err := r.Refresh(r.runContext); err != nil && r.runContext.Err() == nil {
		r.log.Warn(
			r.runContext,
			"gRPC health status refresh failed",
			logger.FromPairs("error", err),
		)
	}
}

// Refresh runs one readiness check and publishes its state to the standard health service.
func (r *Reporter) Refresh(ctx context.Context) error {
	servingStatus := healthpb.HealthCheckResponse_SERVING

	_, err := r.health.Check(ctx, usecasehealth.CheckModeReady)
	if err != nil {
		servingStatus = healthpb.HealthCheckResponse_NOT_SERVING
	}

	for _, service := range r.services {
		r.server.SetServingStatus(service, servingStatus)
	}

	return err
}

// Shutdown marks services not serving and waits for periodic reporting to stop.
func (r *Reporter) Shutdown(ctx context.Context) error {
	r.stopServer.Do(r.server.Shutdown)
	r.cancelRun()

	select {
	case <-r.runStarted:
	default:
		return nil
	}

	select {
	case <-r.runDone:
		return nil
	default:
	}

	select {
	case <-r.runDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop standard gRPC health reporter: %w", ctx.Err())
	}
}
