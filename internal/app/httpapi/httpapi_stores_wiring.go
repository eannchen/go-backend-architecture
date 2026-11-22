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

func (d wiring) buildRedisStores(cacheClient, kvClient *goredis.Client) redisStores {
	return redisStores{
		cacheHealth: rediscachestore.NewHealthStore(cacheClient),
		userCache:   rediscachestore.NewUserCacheStore(cacheClient, d.cfg.Redis.CacheTTL),
		kvHealth:    rediskvstore.NewHealthStore(kvClient),
		session:     rediskvstore.NewSessionStore(kvClient),
		otp:         rediskvstore.NewOTPStore(kvClient),
		oauthState:  rediskvstore.NewOAuthStateStore(kvClient),
		tokenBucket: rediskvstore.NewTokenBucketStore(kvClient),
	}
}
