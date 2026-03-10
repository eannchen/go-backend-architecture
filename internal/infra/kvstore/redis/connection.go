package redis

import (
	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/redisconn"
)

func NewClient(cfg config.RedisConfig) *goredis.Client {
	return redisconn.NewClient(cfg)
}
