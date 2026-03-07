package health

import (
	"context"
	"fmt"

	"vocynex-api/internal/apperr"
	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	"vocynex-api/internal/repository"
)

type Result struct {
	Database Database
}

type Database struct {
	Status        string
	Name          string
	InRecovery    bool
	UptimeSeconds int64
}

type Usecase interface {
	Check(ctx context.Context, mode CheckMode) (Result, error)
}

type impl struct {
	logger      logger.Logger
	tracer      observability.Tracer
	txManager   repository.TxManager
	runtimeRepo repository.RuntimeRepository
}

func New(log logger.Logger, tracer observability.Tracer, runtimeRepo repository.RuntimeRepository, txManager repository.TxManager) Usecase {
	return &impl{
		logger:      log,
		tracer:      tracer,
		txManager:   txManager,
		runtimeRepo: runtimeRepo,
	}
}

func (u *impl) Check(ctx context.Context, mode CheckMode) (result Result, err error) {
	ctx, span := u.tracer.Start(ctx, "usecase", "health_usecase.check")
	defer func() {
		span.Finish(err)
	}()

	if mode == "" {
		mode = CheckModeReady
	}
	if !mode.IsValid() {
		return Result{}, apperr.New(apperr.CodeInvalidArgument, fmt.Sprintf("invalid health check mode: %q", mode))
	}

	result = Result{
		Database: Database{
			Status: "skipped",
		},
	}

	if mode == CheckModeLive {
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
