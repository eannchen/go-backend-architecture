package runtime

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	zaplogger "github.com/eannchen/go-backend-architecture/internal/infra/logger/zap"
	"github.com/eannchen/go-backend-architecture/internal/infra/observability/otel"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

type Application interface{ Shutdown(context.Context) error }

type Runtime struct {
	Config        config.Config
	Logger        logger.Logger
	DBPool        *pgxpool.Pool
	Observability observability.Runtime
}

func New(ctx context.Context) (*Runtime, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	obs, err := otel.Setup(ctx, cfg.OTel, cfg.ServiceName, cfg.AppEnv)
	if err != nil {
		return nil, err
	}
	log, err := zaplogger.New(cfg.Log)
	if err != nil {
		return nil, errutil.Join(err, errutil.Step("shutdown observability after logger init failure", obs.Shutdown(ctx)))
	}
	log.SetLogSink(logEmitterToLogSink(obs.LogEmitter()))
	log.SetContextFieldsProvider(contextFieldsProvider())
	pool, err := postgres.NewPool(ctx, cfg.DB, log)
	if err != nil {
		return nil, errutil.Join(err, errutil.Step("shutdown observability after db init failure", obs.Shutdown(ctx)), errutil.Step("sync logger after db init failure", log.Sync()))
	}
	return &Runtime{Config: cfg, Logger: log, DBPool: pool, Observability: obs}, nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	postgres.ClosePool(ctx, r.DBPool, r.Logger)
	var err error
	if r.Observability != nil {
		err = errors.Join(err, r.Observability.Shutdown(ctx))
	}
	if r.Logger != nil {
		err = errors.Join(err, r.Logger.Sync())
	}
	return err
}

var _ Application = (*Runtime)(nil)
