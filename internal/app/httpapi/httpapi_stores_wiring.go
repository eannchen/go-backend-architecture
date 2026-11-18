package httpapi

import (
	goredis "github.com/redis/go-redis/v9"

	rediscachestore "github.com/eannchen/go-backend-architecture/internal/infra/cache/redis/store"
	rediskvstore "github.com/eannchen/go-backend-architecture/internal/infra/kvstore/redis/store"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type redisStores struct {
	cacheHealth usecasehealth.Pinger
	userCache   *rediscachestore.UserCacheStore
	kvHealth    usecasehealth.Pinger
	session     *rediskvstore.SessionStore
	otp         *rediskvstore.OTPStore
	oauthState  *rediskvstore.OAuthStateStore
	tokenBucket *rediskvstore.TokenBucketStore
}

func (d wiring) buildRedisStores(client *goredis.Client) redisStores {
	return redisStores{
		cacheHealth: rediscachestore.NewHealthStore(client),
		userCache:   rediscachestore.NewUserCacheStore(client, d.cfg.Redis.CacheTTL),
		kvHealth:    rediskvstore.NewHealthStore(client),
		session:     rediskvstore.NewSessionStore(client),
		otp:         rediskvstore.NewOTPStore(client),
		oauthState:  rediskvstore.NewOAuthStateStore(client),
		tokenBucket: rediskvstore.NewTokenBucketStore(client),
	}
}
