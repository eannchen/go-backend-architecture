package runtime

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	"github.com/eannchen/go-backend-architecture/internal/infra/redisconn"
)

type Runtime struct {
	*Telemetry
	Config config.RuntimeConfig
	DBPool *pgxpool.Pool
	// RedisClient is shared by cache and KV stores in the template. Runtime
	// owns the client; stores borrow it and must not close it.
	RedisClient *goredis.Client
}

func New(ctx context.Context, cfg config.RuntimeConfig) (*Runtime, error) {
	telemetry, err := NewTelemetry(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pool, err := postgres.NewPool(ctx, cfg.DB, telemetry.Logger)
	if err != nil {
		cleanupErr := CleanupAfterInitFailure(ctx, cfg.Shutdown.GracePeriod, telemetry.Shutdown)
		return nil, errors.Join(err, WrapCleanupError("shutdown telemetry after db init failure", cleanupErr))
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
	redisErr := r.RedisClient.Close()
	postgres.ClosePool(ctx, r.DBPool, r.Logger)
	return errors.Join(WrapCleanupError("close Redis client", redisErr), r.Telemetry.Shutdown(ctx))
}
