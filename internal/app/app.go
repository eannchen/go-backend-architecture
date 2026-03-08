package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	httpDelivery "go-backend-architecture/internal/delivery/http"
	"go-backend-architecture/internal/infra/config"
	"go-backend-architecture/internal/infra/db/postgres"
	"go-backend-architecture/internal/infra/logger"
	zaplogger "go-backend-architecture/internal/infra/logger/zap"
	"go-backend-architecture/internal/infra/observability"
	otelobs "go-backend-architecture/internal/infra/observability/otel"
	"go-backend-architecture/internal/infra/redisconn"
)

type App struct {
	Config        config.Config
	Logger        logger.Logger
	DBPool        *pgxpool.Pool
	RedisClient   *goredis.Client
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

	wiring := newWiring(cfg, log, tracer)
	redisClient := redisconn.NewClient(cfg.Redis)
	redisStores := wiring.buildRedisStores(redisClient)

	repositories := wiring.buildRepositories(pool, redisStores)
	usecases := wiring.buildUsecases(repositories)
	handlers := wiring.buildHandlers(usecases)
	server, err := wiring.buildServer(handlers)
	if err != nil {
		return nil, joinInitErrors(
			err,
			stepErr("close redis client after server init failure", closeRedisWithError(ctx, redisClient)),
			stepErr("close db pool after server init failure", closePoolWithError(ctx, pool)),
			stepErr("shutdown observability after server init failure", otelRuntime.Shutdown(ctx)),
			stepErr("sync logger after server init failure", log.Sync()),
		)
	}

	return &App{
		Config:        cfg,
		Logger:        log,
		DBPool:        pool,
		RedisClient:   redisClient,
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
	if err := closeRedisWithError(ctx, a.RedisClient); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

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

func closePoolWithError(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	pool.Close()
	return nil
}

func closeRedisWithError(ctx context.Context, client *goredis.Client) error {
	if client == nil {
		return nil
	}
	return client.Close()
}
