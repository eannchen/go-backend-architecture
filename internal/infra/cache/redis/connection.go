package redis

import (
	goredis "github.com/redis/go-redis/v9"

	"go-backend-architecture/internal/infra/config"
)

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
