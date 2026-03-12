package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
	goredis "github.com/redis/go-redis/v9"

	httpdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	healthhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/health"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/observability"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	rediscachestore "github.com/eannchen/go-backend-architecture/internal/infra/cache/redis/store"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	postgresstore "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/store"
	rediskvstore "github.com/eannchen/go-backend-architecture/internal/infra/kvstore/redis/store"
	repocomposite "github.com/eannchen/go-backend-architecture/internal/infra/repository/accountsummary"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/repository"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// wiring centralizes shared dependencies used when wiring constructors.
type wiring struct {
	cfg    config.Config
	log    logger.Logger
	tracer observability.Tracer
	meter  observability.Meter
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

func newWiring(cfg config.Config, log logger.Logger, tracer observability.Tracer, meter observability.Meter) wiring {
	return wiring{
		cfg:    cfg,
		log:    log,
		tracer: tracer,
		meter:  meter,
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
	accountSummaryCachedStore := repocomposite.NewAccountSummaryCachedStore(d.log, d.tracer, accountSummaryStore, redis.accountSummaryCache)
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
		health: usecasehealth.New(d.tracer, d.meter, repos.dbHealthRepo, repos.cacheHealthStore, repos.kvHealthStore),
	}
}

func (d wiring) buildHandlers(responder httpresponse.Responder, usecases appUsecases) appHandlers {
	return appHandlers{
		health: healthhttp.NewHandler(d.log, d.tracer, responder, usecases.health),
	}
}

func (d wiring) buildServer(responder httpresponse.Responder, handlers appHandlers) (*httpdelivery.Server, error) {
	validatorRegistrars := []httpdelivery.ValidationRegistrar{
		healthhttp.RegisterValidation,
	}
	middlewares := []echo.MiddlewareFunc{
		echoMiddleware.Recover(),
		contextmw.NewRequestContextMiddleware(d.cfg.HTTP.ReadTimeout, responder).Handler(),
		observabilitymw.New(d.tracer, d.log).Handler(),
	}
	serverCfg := httpdelivery.ServerConfig{
		Address:      d.cfg.HTTP.Address,
		ReadTimeout:  d.cfg.HTTP.ReadTimeout,
		WriteTimeout: d.cfg.HTTP.WriteTimeout,
		IdleTimeout:  d.cfg.HTTP.IdleTimeout,
	}
	return httpdelivery.NewServer(serverCfg, d.log, validatorRegistrars, middlewares, handlers.health)
}
