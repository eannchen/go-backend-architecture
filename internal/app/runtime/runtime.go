package runtime

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	"github.com/eannchen/go-backend-architecture/internal/infra/redisconn"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

type Application interface{ Shutdown(context.Context) error }

type Runtime struct {
	*Telemetry
	Config      config.RuntimeConfig
	DBPool      *pgxpool.Pool
	RedisClient *goredis.Client
}

func New(ctx context.Context, cfg config.RuntimeConfig) (*Runtime, error) {
	telemetry, err := NewTelemetry(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pool, err := postgres.NewPool(ctx, cfg.DB, telemetry.Logger)
	if err != nil {
		return nil, errutil.Join(err, errutil.Step("shutdown telemetry after db init failure", telemetry.Shutdown(ctx)))
	}
	redisClient := redisconn.NewClient(cfg.Redis)
	return &Runtime{
		Telemetry:   telemetry,
		Config:      cfg,
		DBPool:      pool,
		RedisClient: redisClient,
	}, nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	var err error
	if r.RedisClient != nil {
		err = errors.Join(err, r.RedisClient.Close())
	}
	postgres.ClosePool(ctx, r.DBPool, r.Logger)
	err = errors.Join(err, r.Telemetry.Shutdown(ctx))
	return err
}

var _ Application = (*Runtime)(nil)
