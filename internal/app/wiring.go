package app

import (
	"github.com/jackc/pgx/v5/pgxpool"

	httpdelivery "vocynex-api/internal/delivery/http"
	healthhttp "vocynex-api/internal/delivery/http/health"
	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/db/postgres"
	"vocynex-api/internal/infra/db/postgres/repos"
	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
	"vocynex-api/internal/repository"
	usecasehealth "vocynex-api/internal/usecase/health"
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
	health usecasehealth.Usecase
}

type appHandlers struct {
	health httpdelivery.RouteRegistrar
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
		health: usecasehealth.New(d.log, d.tracer, repos.runtimeRepo, repos.txManager),
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
