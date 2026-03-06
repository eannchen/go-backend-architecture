package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	httpDelivery "vocynex-api/internal/delivery/http"
	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/db/postgres"
	"vocynex-api/internal/infra/db/postgres/repos"
	"vocynex-api/internal/infra/logger"
	zaplogger "vocynex-api/internal/infra/logger/zap"
	"vocynex-api/internal/infra/observability"
	otelobs "vocynex-api/internal/infra/observability/otel"
	"vocynex-api/internal/usecase"
)

type App struct {
	Config        config.Config
	Logger        logger.Logger
	DBPool        *pgxpool.Pool
	Server        *httpDelivery.Server
	Observability observability.Runtime
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	otelRuntime, err := otelobs.Setup(ctx, cfg.OTel, cfg.ServiceName, cfg.AppEnv)
	if err != nil {
		return nil, err
	}
	tracer := otelRuntime.Tracer()

	log, err := zaplogger.New(cfg.Log)
	if err != nil {
		return nil, joinInitErrors(
			err,
			stepErr("shutdown observability after logger init failure", otelRuntime.Shutdown(ctx)),
		)
	}
	log.SetLogSink(logEmitterToLogSink(otelRuntime.LogEmitter()))
	log.SetContextFieldsProvider(contextFieldsProvider())

	pool, err := postgres.NewPool(ctx, cfg.DB, log)
	if err != nil {
		return nil, joinInitErrors(
			err,
			stepErr("shutdown observability after db init failure", otelRuntime.Shutdown(ctx)),
			stepErr("sync logger after db init failure", log.Sync()),
		)
	}

	runtimeRepo := repos.NewRuntimeRepository(pool, tracer)
	txManager := postgres.NewTxManager(pool, tracer)
	healthUsecase := usecase.NewHealthUsecase(runtimeRepo, txManager, log, tracer)
	healthHandler := httpDelivery.NewHealthHandler(healthUsecase, log, tracer)
	server := httpDelivery.NewServer(cfg.HTTP, log, tracer, healthHandler)

	return &App{
		Config:        cfg,
		Logger:        log,
		DBPool:        pool,
		Server:        server,
		Observability: otelRuntime,
	}, nil
}

func (a *App) Start() error {
	return a.Server.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	var shutdownErr error

	if err := a.Server.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	postgres.ClosePool(ctx, a.DBPool, a.Logger)

	if a.Observability != nil {
		if err := a.Observability.Shutdown(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	if err := a.Logger.Sync(); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	return shutdownErr
}

func stepErr(step string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", step, err)
}

func joinInitErrors(base error, cleanupErrs ...error) error {
	all := make([]error, 0, len(cleanupErrs)+1)
	all = append(all, base)
	all = append(all, cleanupErrs...)
	return errors.Join(all...)
}
