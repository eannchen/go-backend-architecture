package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/logger"
)

func NewPool(ctx context.Context, cfg config.DBConfig, log logger.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Info(ctx, "database connection established")
	return pool, nil
}

func ClosePool(ctx context.Context, pool *pgxpool.Pool, log logger.Logger) {
	if pool == nil {
		return
	}
	start := time.Now()
	pool.Close()
	log.Info(ctx, "database pool closed", logger.FromPairs("duration_ms", time.Since(start).Milliseconds()))
}
