package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	httpdelivery "go-backend-architecture/internal/delivery/http"
	healthhttp "go-backend-architecture/internal/delivery/http/health"
	rediscachestore "go-backend-architecture/internal/infra/cache/redis/store"
	"go-backend-architecture/internal/infra/config"
	"go-backend-architecture/internal/infra/db/postgres"
	postgresstore "go-backend-architecture/internal/infra/db/postgres/store"
	rediskvstore "go-backend-architecture/internal/infra/kvstore/redis/store"
	"go-backend-architecture/internal/infra/logger"
	"go-backend-architecture/internal/infra/observability"
	repoimplaccountsummary "go-backend-architecture/internal/infra/repoimpl/accountsummary"
	"go-backend-architecture/internal/repository"
	usecasehealth "go-backend-architecture/internal/usecase/health"
)

// wiring centralizes shared dependencies used when wiring constructors.
type wiring struct {
	cfg    config.Config
	log    logger.Logger
	tracer observability.Tracer
}

type appRepositories struct {
	dbHealthRepo             repository.DBHealthRepository
	accountSummaryRepo       repository.AccountSummaryRepository
	accountSummaryCachedRepo repository.AccountSummaryRepository
	txManager                repository.TxManager
	cacheHealthStore         repository.CacheHealthStore
	kvHealthStore            repository.KVHealthStore
}

type appUsecases struct {
	health usecasehealth.Usecase
}

type appHandlers struct {
	health httpdelivery.RouteRegistrar
}

type redisStores struct {
	accountSummaryCache *rediscachestore.AccountSummaryStore
	cacheHealth         repository.CacheHealthStore
	kvHealth            repository.KVHealthStore
}

func newWiring(cfg config.Config, log logger.Logger, tracer observability.Tracer) wiring {
	return wiring{
		cfg:    cfg,
		log:    log,
		tracer: tracer,
	}
}

func (d wiring) buildRedisStores(client *goredis.Client) redisStores {
	return redisStores{
		cacheHealth:         rediscachestore.NewHealthStore(client),
		kvHealth:            rediskvstore.NewHealthStore(client),
		accountSummaryCache: rediscachestore.NewAccountSummaryStore(client, d.cfg.Redis.CacheTTL),
	}
}

func (d wiring) buildRepositories(pool *pgxpool.Pool, redis redisStores) appRepositories {
	dbHealthStore := postgresstore.NewDBHealthStore(pool, d.tracer)
	accountSummaryStore := postgresstore.NewAccountSummaryStore(pool, d.tracer)
	accountSummaryCachedStore := repoimplaccountsummary.NewAccountSummaryCachedStore(d.log, d.tracer, accountSummaryStore, redis.accountSummaryCache)
	return appRepositories{
		txManager:                postgres.NewTxManager(pool, d.tracer),
		dbHealthRepo:             dbHealthStore,
		cacheHealthStore:         redis.cacheHealth,
		kvHealthStore:            redis.kvHealth,
		accountSummaryRepo:       accountSummaryStore,
		accountSummaryCachedRepo: accountSummaryCachedStore,
	}
}

func (d wiring) buildUsecases(repos appRepositories) appUsecases {
	return appUsecases{
		health: usecasehealth.New(d.log, d.tracer, repos.dbHealthRepo, repos.cacheHealthStore, repos.kvHealthStore),
	}
}

func (d wiring) buildHandlers(usecases appUsecases) appHandlers {
	return appHandlers{
		health: healthhttp.NewHandler(d.log, d.tracer, usecases.health),
	}
}

func (d wiring) buildServer(handlers appHandlers) (*httpdelivery.Server, error) {
	validatorRegistrars := []httpdelivery.ValidationRegistrar{
		healthhttp.RegisterValidation,
	}
	return httpdelivery.NewServer(d.cfg.HTTP, d.log, d.tracer, validatorRegistrars, handlers.health)
}
