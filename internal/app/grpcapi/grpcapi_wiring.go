package grpcapi

import (
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type wiring struct {
	cfg    config.Config
	log    logger.Logger
	tracer observability.Tracer
	meter  observability.Meter
}

func newWiring(cfg config.Config, log logger.Logger, tracer observability.Tracer, meter observability.Meter) wiring {
	return wiring{
		cfg:    cfg,
		log:    log,
		tracer: tracer,
		meter:  meter,
	}
}
