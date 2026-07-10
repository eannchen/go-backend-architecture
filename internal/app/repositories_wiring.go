package app

import (
	"github.com/jackc/pgx/v5/pgxpool"

	composeduser "github.com/eannchen/go-backend-architecture/internal/infra/composed/user"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	postgresstore "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/store"
	repocache "github.com/eannchen/go-backend-architecture/internal/repository/cache"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

type appRepositories struct {
	dbHealthRepo     repodb.DBHealthRepository
	txManager        repodb.TxManager
	cacheHealthStore repocache.CacheHealthStore
	kvHealthStore    repokvstore.KVHealthStore
	userRepo         repodb.UserRepository
	sessionRepo      repokvstore.SessionRepository
	otpRepo          repokvstore.OTPRepository
	oauthStateRepo   repokvstore.OAuthStateRepository
}

func (d wiring) buildRepositories(pool *pgxpool.Pool, redis redisStores) appRepositories {
	dbUserRepo := postgresstore.NewUserStore(pool, d.tracer)
	userRepo := composeduser.NewCachedUserStore(d.log, d.tracer, dbUserRepo, redis.userCache)

	return appRepositories{
		txManager:        postgres.NewTxManager(pool, d.tracer),
		dbHealthRepo:     postgresstore.NewDBHealthStore(pool, d.tracer),
		cacheHealthStore: redis.cacheHealth,
		kvHealthStore:    redis.kvHealth,
		userRepo:         userRepo,
		sessionRepo:      redis.session,
		otpRepo:          redis.otp,
		oauthStateRepo:   redis.oauthState,
	}
}
