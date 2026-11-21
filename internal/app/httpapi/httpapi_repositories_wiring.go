package httpapi

import (
	"github.com/jackc/pgx/v5/pgxpool"

	composeduser "github.com/eannchen/go-backend-architecture/internal/infra/composed/user"
	postgresstore "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/store"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
	globalratelimit "github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type appRepositories struct {
	dbHealthRepo     usecasehealth.DatabaseHealth
	cacheHealthStore usecasehealth.Pinger
	kvHealthStore    usecasehealth.Pinger
	userRepo         *composeduser.CachedUserStore
	sessionRepo      authsession.SessionRepository
	otpRepo          authotp.OTPRepository
	oauthStateRepo   authoauth.OAuthStateRepository
	tokenBucketRepo  globalratelimit.TokenBucketRepository
}

func (d wiring) buildRepositories(pool *pgxpool.Pool, redis redisStores) appRepositories {
	dbUserRepo := postgresstore.NewUserStore(pool, d.tracer)
	userRepo := composeduser.NewCachedUserStore(d.log, d.tracer, dbUserRepo, redis.userCache)

	return appRepositories{
		dbHealthRepo:     postgresstore.NewDBHealthStore(pool, d.tracer),
		cacheHealthStore: redis.cacheHealth,
		kvHealthStore:    redis.kvHealth,
		userRepo:         userRepo,
		sessionRepo:      redis.session,
		otpRepo:          redis.otp,
		oauthStateRepo:   redis.oauthState,
		tokenBucketRepo:  redis.tokenBucket,
	}
}
