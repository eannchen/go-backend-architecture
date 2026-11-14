package kafkaworker

import (
	"context"
	"errors"
	"slices"
	"strings"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	otelinfra "github.com/eannchen/go-backend-architecture/internal/infra/observability/otel"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

type App struct {
	*appruntime.Telemetry
	Config  config.KafkaWorkerConfig
	workers []appruntime.Worker
	done    chan struct{}
}

func New(ctx context.Context) (_ *App, err error) {
	cfg, err := config.LoadKafkaWorker()
	if err != nil {
		return nil, err
	}
	telemetry, err := appruntime.NewTelemetry(ctx, cfg.RuntimeConfig)
	if err != nil {
		return nil, err
	}
	// Record resources as they are built, then unwind in reverse order if a
	// later TLS load, reader constructor, or handler wiring step fails.
	shutdowns := []func(context.Context) error{telemetry.Shutdown}
	defer func() {
		if err != nil {
			slices.Reverse(shutdowns)
			err = errors.Join(err, appruntime.CleanupAfterInitFailure(ctx, cfg.Shutdown.GracePeriod, shutdowns...))
		}
	}()
	app := &App{Telemetry: telemetry, Config: cfg, done: make(chan struct{})}
	for _, readerConfig := range cfg.Readers {
		tlsConfig, err := buildTLSConfig(readerConfig.Kafka.Security.TLS)
		if err != nil {
			return nil, err
		}
		reader, err := kafkaclient.NewReader(readerConfig.Kafka, tlsConfig,
			otelinfra.KafkaHooks(telemetry.Observability.TracerProvider(), telemetry.Observability.MeterProvider(), readerConfig.Kafka.GroupID)...)
		if err != nil {
			return nil, err
		}
		shutdowns = append(shutdowns, reader.Close)
		worker, err := buildWorker(reader, telemetry, readerConfig, cfg.Routes)
		if err != nil {
			return nil, err
		}
		app.workers = append(app.workers, appruntime.Worker{
			Name:     strings.Join(readerConfig.Kafka.Topics, ","),
			Run:      worker.Run,
			Shutdown: reader.Close,
		})
	}
	return app, nil
}

// Start owns every worker loop until processing and reader cleanup finish.
func (a *App) Start(ctx context.Context) error {
	defer close(a.done)
	for _, reader := range a.Config.Readers {
		a.Logger.Info(ctx, "Kafka worker starting", logger.FromPairs(
			"kafka.topics", reader.Kafka.Topics, "kafka.group_id", reader.Kafka.GroupID,
			"worker.failure_policy", string(a.Config.FailurePolicy)))
	}
	return (appruntime.WorkerGroup{
		Workers:         a.workers,
		StopOnError:     a.Config.FailurePolicy == config.KafkaWorkersStopAll,
		ShutdownTimeout: a.Config.Shutdown.GracePeriod,
		Logger:          a.Logger,
	}).Run(ctx)
}

func (a *App) Shutdown(ctx context.Context) error {
	// Each worker closes its own reader after its loop stops. Keep telemetry
	// alive until every reader's final commit/acknowledgment cleanup has finished.
	select {
	case <-a.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return a.Telemetry.Shutdown(ctx)
}
