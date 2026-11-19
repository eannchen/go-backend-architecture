package grpcapi

import (
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/pkg/logger"
)

type wiring struct {
	cfg    config.GRPCAPIConfig
	log    logger.Logger
	tracer observability.Tracer
	meter  observability.Meter
}

func newWiring(cfg config.GRPCAPIConfig, log logger.Logger, tracer observability.Tracer, meter observability.Meter) wiring {
	return wiring{
		cfg:    cfg,
		log:    log,
		tracer: tracer,
		meter:  meter,
	}
}
