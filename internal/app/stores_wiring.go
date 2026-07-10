package app

import (
	goredis "github.com/redis/go-redis/v9"

	rediscachestore "github.com/eannchen/go-backend-architecture/internal/infra/cache/redis/store"
	rediskvstore "github.com/eannchen/go-backend-architecture/internal/infra/kvstore/redis/store"
	repocache "github.com/eannchen/go-backend-architecture/internal/repository/cache"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

type redisStores struct {
	cacheHealth repocache.CacheHealthStore
	userCache   *rediscachestore.UserCacheStore
	kvHealth    repokvstore.KVHealthStore
	session     *rediskvstore.SessionStore
	otp         *rediskvstore.OTPStore
	oauthState  *rediskvstore.OAuthStateStore
}

func (d wiring) buildRedisStores(client *goredis.Client) redisStores {
	return redisStores{
		cacheHealth: rediscachestore.NewHealthStore(client),
		userCache:   rediscachestore.NewUserCacheStore(client, d.cfg.Redis.CacheTTL),
		kvHealth:    rediskvstore.NewHealthStore(client),
		session:     rediskvstore.NewSessionStore(client),
		otp:         rediskvstore.NewOTPStore(client),
		oauthState:  rediskvstore.NewOAuthStateStore(client),
	}
}
