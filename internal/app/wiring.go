package app

import (
	"github.com/jackc/pgx/v5/pgxpool"

	httpDelivery "vocynex-api/internal/delivery/http"
	healthHTTP "vocynex-api/internal/delivery/http/health"
	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/db/postgres"
	"vocynex-api/internal/infra/db/postgres/repos"
	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	"vocynex-api/internal/repository"
	usecaseHealth "vocynex-api/internal/usecase/health"
)

// wiring centralizes shared dependencies used when wiring constructors.
type wiring struct {
	cfg    config.Config
	log    logger.Logger
	tracer observability.Tracer
}

type appRepositories struct {
	runtimeRepo repository.RuntimeRepository
	txManager   repository.TxManager
}

type appUsecases struct {
	health *usecaseHealth.Usecase
}

type appHandlers struct {
	health *healthHTTP.Handler
}

func newWiring(cfg config.Config, log logger.Logger, tracer observability.Tracer) wiring {
	return wiring{
		cfg:    cfg,
		log:    log,
		tracer: tracer,
	}
}

func (d wiring) buildRepositories(pool *pgxpool.Pool) appRepositories {
	return appRepositories{
		txManager:   postgres.NewTxManager(pool, d.tracer),
		runtimeRepo: repos.NewRuntimeRepository(pool, d.tracer),
	}
}

func (d wiring) buildUsecases(repos appRepositories) appUsecases {
	return appUsecases{
		health: usecaseHealth.New(d.log, d.tracer, repos.runtimeRepo, repos.txManager),
	}
}

func (d wiring) buildHandlers(usecases appUsecases) appHandlers {
	return appHandlers{
		health: healthHTTP.NewHandler(d.log, d.tracer, usecases.health),
	}
}

func (d wiring) buildServer(handlers appHandlers) *httpDelivery.Server {
	return httpDelivery.NewServer(d.cfg.HTTP, d.log, d.tracer, handlers.health)
}
