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
	Dependencies map[string]string
	Details      map[string]any
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
		return HealthCheckResult{}, apperr.New(
			apperr.CodeInvalidArgument,
			fmt.Sprintf("invalid health check mode: %q", mode),
			apperr.Field("check", mode),
		)
	}

	result = HealthCheckResult{
		Dependencies: map[string]string{
			"database":        "skipped",
			"database_server": "skipped",
			"database_tx":     "skipped",
		},
	}

	if mode == HealthCheckLive {
		return result, nil
	}

	// Quick non-transactional readiness check (simple sqlc static query).
	if err := u.runtimeRepo.Ping(ctx); err != nil {
		result.Dependencies["database"] = "down"
		return result, apperr.Wrap(
			err,
			apperr.CodeUnavailable,
			"database readiness failed",
			apperr.Field("dependency", "database"),
		)
	}
	result.Dependencies["database"] = "up"

	serverStatus, err := u.runtimeRepo.GetServerStatus(ctx)
	if err != nil {
		result.Dependencies["database_server"] = "down"
		return result, apperr.Wrap(
			err,
			apperr.CodeUnavailable,
			"database server status query failed",
			apperr.Field("dependency", "database_server"),
		)
	}
	result.Dependencies["database_server"] = "up"
	result.Details = map[string]any{
		"database": map[string]any{
			"name":           serverStatus.DatabaseName,
			"in_recovery":    serverStatus.InRecovery,
			"uptime_seconds": serverStatus.UptimeSeconds,
		},
	}

	return result, nil
}
