package redisconn

import (
	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
)

// NewClient creates a Redis client with its own connection pool. Each call creates
// a separate pool, even with identical settings. App composition chooses whether
// cache and KV stores share a client and owns closing each client once.
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
