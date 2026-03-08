package redis

import (
	goredis "github.com/redis/go-redis/v9"

	"go-backend-architecture/internal/infra/config"
	"go-backend-architecture/internal/infra/redisconn"
)

func NewClient(cfg config.RedisConfig) *goredis.Client {
	return redisconn.NewClient(cfg)
}
