package redisconn

import (
	goredis "github.com/redis/go-redis/v9"

	"go-backend-architecture/internal/infra/config"
)

// NewClient builds a shared Redis client from project config.
// Callers in cache/redis and kvstore/redis can reuse the same instance.
func NewClient(cfg config.RedisConfig) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
}
