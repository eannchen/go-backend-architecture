package runtime

import (
	"context"
	"errors"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	zaplogger "github.com/eannchen/go-backend-architecture/internal/infra/logger/zap"
	"github.com/eannchen/go-backend-architecture/internal/infra/observability/otel"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

// Telemetry owns the process logger and observability providers without data services.
type Telemetry struct {
	Logger        logger.Logger
	Observability observability.Runtime
}

// NewTelemetry creates process telemetry with a caller-selected service identity.
func NewTelemetry(ctx context.Context, cfg config.Config, serviceName string) (*Telemetry, error) {
	obs, err := otel.Setup(ctx, cfg.OTel, serviceName, cfg.AppEnv)
	if err != nil {
		return nil, err
	}
	log, err := zaplogger.New(cfg.Log)
	if err != nil {
		return nil, errutil.Join(err, errutil.Step("shutdown observability after logger init failure", obs.Shutdown(ctx)))
	}
	log.SetLogSink(logEmitterToLogSink(obs.LogEmitter()))
	log.SetContextFieldsProvider(contextFieldsProvider(obs.Tracer()))
	return &Telemetry{Logger: log, Observability: obs}, nil
}

// Shutdown flushes exported telemetry before syncing the local logger.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	var err error
	if t.Observability != nil {
		err = errors.Join(err, errutil.Step("shutdown observability", t.Observability.Shutdown(ctx)))
	}
	if t.Logger != nil {
		err = errors.Join(err, errutil.Step("sync logger", t.Logger.Sync()))
	}
	return err
}
