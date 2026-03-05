package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	httpDelivery "vocynex-api/internal/delivery/http"
	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/db/postgres"
	"vocynex-api/internal/infra/db/postgres/repos"
	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	"vocynex-api/internal/usecase"
)

type App struct {
	Config            config.Config
	Logger            logger.Logger
	DBPool            *pgxpool.Pool
	Server            *httpDelivery.Server
	ShutdownTelemetry observability.ShutdownFunc
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	log, err := logger.NewZap(cfg.Log)
	if err != nil {
		return nil, err
	}

	otelShutdown, otelLogEmitter, err := observability.Setup(ctx, cfg.OTel, cfg.ServiceName, cfg.AppEnv)
	if err != nil {
		_ = log.Sync()
		return nil, err
	}
	log.SetLogSink(newObservabilityLogSink(otelLogEmitter))
	log.SetContextFieldsProvider(newContextFieldsProvider())
	tracer := observability.NewTracer(cfg.ServiceName)

	pool, err := postgres.NewPool(ctx, cfg.DB, log)
	if err != nil {
		_ = otelShutdown(ctx)
		_ = log.Sync()
		return nil, err
	}

	runtimeRepo := repos.NewRuntimeRepository(pool, tracer)
	txManager := postgres.NewTxManager(pool, tracer)
	healthUsecase := usecase.NewHealthUsecase(runtimeRepo, txManager, log, tracer)
	healthHandler := httpDelivery.NewHealthHandler(healthUsecase, log, tracer)
	server := httpDelivery.NewServer(cfg.HTTP, log, tracer, healthHandler)

	return &App{
		Config:            cfg,
		Logger:            log,
		DBPool:            pool,
		Server:            server,
		ShutdownTelemetry: otelShutdown,
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

	if a.ShutdownTelemetry != nil {
		if err := a.ShutdownTelemetry(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	if err := a.Logger.Sync(); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	return shutdownErr
}
