package kafkaworker

import (
	"context"
	"errors"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

type App struct {
	*appruntime.Telemetry
	Config config.KafkaWorkerConfig
	reader *kafkaclient.Reader
	worker *kafkadelivery.NotificationWorker
	done   chan struct{}
}

var _ appruntime.Application = (*App)(nil)

func New(ctx context.Context) (*App, error) {
	cfg, err := config.LoadKafkaWorker()
	if err != nil {
		return nil, err
	}
	// Keep shared runtime configuration, but construct only resources this process
	// uses. NewTelemetry reuses runtime logging/tracing without opening DB/Redis.
	telemetry, err := appruntime.NewTelemetry(ctx, cfg.RuntimeConfig)
	if err != nil {
		return nil, err
	}
	reader, err := kafkaclient.NewReader(cfg.Kafka)
	if err != nil {
		cleanupErr := appruntime.CleanupAfterInitFailure(ctx, cfg.Shutdown.GracePeriod, telemetry.Shutdown)
		return nil, errors.Join(err, errutil.Step("shutdown telemetry after Kafka init failure", cleanupErr))
	}
	usecase := notification.New(telemetry.Logger)
	worker := kafkadelivery.NewNotificationWorker(reader, usecase, telemetry.Observability.Tracer())
	return &App{Telemetry: telemetry, Config: cfg, reader: reader, worker: worker, done: make(chan struct{})}, nil
}

// Start is called once with the process's signal-aware root context.
func (a *App) Start(ctx context.Context) error {
	defer close(a.done)
	a.Logger.Info(ctx, "Kafka worker started", logger.FromPairs("kafka.topic", a.Config.Kafka.Topic, "kafka.group_id", a.Config.Kafka.GroupID))
	return a.worker.Run(ctx)
}

func (a *App) Shutdown(ctx context.Context) error {
	// Root cancellation stops fetching/handling first. Wait before closing the
	// reader and flushing telemetry, so no handler uses a resource being shut down.
	select {
	case <-a.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return errors.Join(a.reader.Close(ctx), a.Telemetry.Shutdown(ctx))
}
