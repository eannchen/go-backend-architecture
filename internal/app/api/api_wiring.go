package api

import (
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// wiring centralizes shared dependencies used when wiring constructors.
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
