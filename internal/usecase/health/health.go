package health

import (
	"context"
	"fmt"

	"go-backend-architecture/internal/apperr"
	"go-backend-architecture/internal/logger"
	"go-backend-architecture/internal/observability"
	"go-backend-architecture/internal/repository"
)

type Result struct {
	Database Database
	Cache    Dependency
	KVStore  Dependency
}

type Database struct {
	Status        string
	Name          string
	InRecovery    bool
	UptimeSeconds int64
}

type Dependency struct {
	Status string
}

type Usecase interface {
	Check(ctx context.Context, mode CheckMode) (Result, error)
}

type impl struct {
	logger      logger.Logger
	tracer      observability.Tracer
	dbHealth    repository.DBHealthRepository
	cacheHealth repository.CacheHealthStore
	kvHealth    repository.KVHealthStore
}

func New(
	log logger.Logger,
	tracer observability.Tracer,
	dbHealth repository.DBHealthRepository,
	cacheHealth repository.CacheHealthStore,
	kvHealth repository.KVHealthStore,
) Usecase {
	return &impl{
		logger:      log,
		tracer:      tracer,
		dbHealth:    dbHealth,
		cacheHealth: cacheHealth,
		kvHealth:    kvHealth,
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
		Cache: Dependency{
			Status: "skipped",
		},
		KVStore: Dependency{
			Status: "skipped",
		},
	}

	if mode == CheckModeLive {
		return result, nil
	}

	if err := u.dbHealth.Ping(ctx); err != nil {
		result.Database.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "database readiness failed")
	}

	serverStatus, err := u.dbHealth.GetServerStatus(ctx)
	if err != nil {
		result.Database.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "database server status query failed")
	}

	result.Database.Status = "up"
	result.Database.Name = serverStatus.DatabaseName
	result.Database.InRecovery = serverStatus.InRecovery
	result.Database.UptimeSeconds = serverStatus.UptimeSeconds

	if err := u.cacheHealth.Ping(ctx); err != nil {
		result.Cache.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "cache readiness failed")
	}
	result.Cache.Status = "up"

	if err := u.kvHealth.Ping(ctx); err != nil {
		result.KVStore.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "kv readiness failed")
	}
	result.KVStore.Status = "up"

	return result, nil
}
