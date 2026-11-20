package runtime

import (
	"context"
	"errors"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/pkg/logger"
	zaplogger "github.com/eannchen/go-backend-architecture/pkg/logger/zap"
	"github.com/eannchen/go-backend-architecture/pkg/observability/otel"
)

// Telemetry owns the process logger and observability providers without data services.
type Telemetry struct {
	Logger        logger.Logger
	Observability *otel.Runtime
}

// NewTelemetry creates process telemetry with the configured service identity.
func NewTelemetry(ctx context.Context, cfg config.RuntimeConfig) (*Telemetry, error) {
	obs, err := otel.Setup(ctx, otel.Config{
		ExportEnabled:      cfg.OTel.ExportEnabled,
		TracesEndpoint:     cfg.OTel.TracesEndpoint,
		LogsEndpoint:       cfg.OTel.LogsEndpoint,
		MetricsEndpoint:    cfg.OTel.MetricsEndpoint,
		Insecure:           cfg.OTel.Insecure,
		TraceSamplingRatio: cfg.OTel.TraceSamplingRatio,
	}, cfg.ServiceName, cfg.AppEnv)
	if err != nil {
		return nil, err
	}
	log, err := zaplogger.New(zaplogger.Config{
		Level:       cfg.Log.Level,
		SinkLevel:   cfg.Log.OTELevel,
		Development: cfg.Log.Development,
	})
	if err != nil {
		cleanupErr := CleanupAfterInitFailure(ctx, cfg.Shutdown.GracePeriod, obs.Shutdown)
		return nil, errors.Join(err, WrapCleanupError("shutdown observability after logger init failure", cleanupErr))
	}
	log.SetLogSink(logEmitterToLogSink(obs.LogEmitter()))
	log.SetContextFieldsProvider(contextFieldsProvider(obs.Tracer()))
	return &Telemetry{Logger: log, Observability: obs}, nil
}

// Shutdown flushes exported telemetry before syncing the local logger.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	return errors.Join(
		WrapCleanupError("shutdown observability", t.Observability.Shutdown(ctx)),
		WrapCleanupError("sync logger", t.Logger.Sync()),
	)
}
