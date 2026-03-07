package usecase

import (
	"context"
	"fmt"

	"vocynex-api/internal/apperr"
	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	"vocynex-api/internal/repository"
)

type HealthCheckResult struct {
	Database HealthDatabase
}

type HealthDatabase struct {
	Status        string
	Name          string
	InRecovery    bool
	UptimeSeconds int64
}

type HealthCheckMode string

const (
	HealthCheckLive  HealthCheckMode = "live"
	HealthCheckReady HealthCheckMode = "ready"
)

func (m HealthCheckMode) IsValid() bool {
	switch m {
	case HealthCheckLive, HealthCheckReady:
		return true
	default:
		return false
	}
}

func ParseHealthCheckMode(raw string) (HealthCheckMode, error) {
	if raw == "" {
		return HealthCheckReady, nil
	}

	mode := HealthCheckMode(raw)
	if !mode.IsValid() {
		return "", fmt.Errorf("invalid check mode %q, allowed: live, ready", raw)
	}
	return mode, nil
}

type HealthChecker interface {
	Check(ctx context.Context, mode HealthCheckMode) (HealthCheckResult, error)
}

type HealthUsecase struct {
	runtimeRepo repository.RuntimeRepository
	txManager   repository.TxManager
	logger      logger.Logger
	tracer      observability.Tracer
}

func NewHealthUsecase(log logger.Logger, tracer observability.Tracer, runtimeRepo repository.RuntimeRepository, txManager repository.TxManager) *HealthUsecase {
	return &HealthUsecase{
		runtimeRepo: runtimeRepo,
		txManager:   txManager,
		logger:      log,
		tracer:      tracer,
	}
}

func (u *HealthUsecase) Check(ctx context.Context, mode HealthCheckMode) (result HealthCheckResult, err error) {
	ctx, span := u.tracer.Start(ctx, "usecase", "health_usecase.check")
	defer func() {
		span.Finish(err)
	}()

	if mode == "" {
		mode = HealthCheckReady
	}
	if !mode.IsValid() {
		return HealthCheckResult{},
			apperr.New(apperr.CodeInvalidArgument, fmt.Sprintf("invalid health check mode: %q", mode))
	}

	result = HealthCheckResult{
		Database: HealthDatabase{
			Status: "skipped",
		},
	}

	if mode == HealthCheckLive {
		return result, nil
	}

	if err := u.runtimeRepo.Ping(ctx); err != nil {
		result.Database.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "database readiness failed")
	}

	serverStatus, err := u.runtimeRepo.GetServerStatus(ctx)
	if err != nil {
		result.Database.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "database server status query failed")
	}

	result.Database.Status = "up"
	result.Database.Name = serverStatus.DatabaseName
	result.Database.InRecovery = serverStatus.InRecovery
	result.Database.UptimeSeconds = serverStatus.UptimeSeconds

	return result, nil
}
