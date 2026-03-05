package usecase

import (
	"context"
	"errors"

	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	"vocynex-api/internal/repository"
)

type HealthStatus struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies"`
}

type HealthChecker interface {
	Check(ctx context.Context) (HealthStatus, error)
}

type HealthUsecase struct {
	runtimeRepo repository.RuntimeRepository
	txManager   repository.TxManager
	logger      logger.Logger
	tracer      observability.Tracer
}

func NewHealthUsecase(runtimeRepo repository.RuntimeRepository, txManager repository.TxManager, log logger.Logger, tracer observability.Tracer) *HealthUsecase {
	return &HealthUsecase{
		runtimeRepo: runtimeRepo,
		txManager:   txManager,
		logger:      log,
		tracer:      tracer,
	}
}

func (u *HealthUsecase) Check(ctx context.Context) (HealthStatus, error) {
	ctx, span := u.tracer.Start(ctx, "usecase", "health_usecase.check")
	defer span.End()

	status := HealthStatus{
		Status: "ok",
		Dependencies: map[string]string{
			"database":               "up",
			"database_tx":            "up",
			"database_dynamic_query": "up",
			"database_static_query":  "up",
		},
	}

	// Quick non-transactional readiness check (simple sqlc static query).
	if err := u.runtimeRepo.Ping(ctx); err != nil {
		status.Status = "degraded"
		status.Dependencies["database"] = "down"
		span.Fail(err, "database ping failed")
		u.logger.Warn(ctx, "health check failed on database ping")
		return status, errors.Join(errors.New("database readiness failed"), err)
	}

	// Transactional check demonstrates cross-repository extensibility:
	// today it uses Runtime(), later it can use Runtime()+User()+Order() in one tx.
	txErr := u.txManager.WithTx(ctx, func(txCtx context.Context, repos repository.TxRepository) error {
		// Validate dynamic query path (Squirrel) within same tx.
		_, err := repos.Runtime().SearchRuntimeValues(txCtx, "", 1)
		return err
	})
	if txErr != nil {
		status.Status = "degraded"
		status.Dependencies["database_tx"] = "down"
		status.Dependencies["database_dynamic_query"] = "down"
		status.Dependencies["database_static_query"] = "down"
		span.Fail(txErr, "transactional health checks failed")
		u.logger.Warn(ctx, "health check failed on transactional database checks")
		return status, errors.Join(errors.New("database transactional health check failed"), txErr)
	}

	span.OK()
	u.logger.Debug(ctx, "health check passed")
	return status, nil
}
