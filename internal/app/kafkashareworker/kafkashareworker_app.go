package kafkashareworker

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
	Config config.KafkaShareWorkerConfig
	reader *kafkaclient.ShareReader
	worker *kafkadelivery.NotificationShareWorker
	done   chan struct{}
}

var _ appruntime.Application = (*App)(nil)

func New(ctx context.Context) (*App, error) {
	cfg, err := config.LoadKafkaShareWorker()
	if err != nil {
		return nil, err
	}
	telemetry, err := appruntime.NewTelemetry(ctx, cfg.RuntimeConfig)
	if err != nil {
		return nil, err
	}
	reader, err := kafkaclient.NewShareReader(cfg.Kafka)
	if err != nil {
		cleanupErr := appruntime.CleanupAfterInitFailure(ctx, cfg.Shutdown.GracePeriod, telemetry.Shutdown)
		return nil, errors.Join(err, errutil.Step("shutdown telemetry after share reader init failure", cleanupErr))
	}
	worker, err := kafkadelivery.NewNotificationShareWorker(shareReaderAdapter{reader}, notification.New(telemetry.Logger), telemetry.Logger, kafkadelivery.ShareWorkerOptions{
		Concurrency: cfg.Concurrency, HandlingTimeout: cfg.HandlingTimeout, AcknowledgmentMargin: cfg.AcknowledgmentMargin,
	})
	if err != nil {
		cleanupErr := appruntime.CleanupAfterInitFailure(ctx, cfg.Shutdown.GracePeriod, func(cleanupCtx context.Context) error {
			return errors.Join(reader.Close(cleanupCtx), telemetry.Shutdown(cleanupCtx))
		})
		return nil, errors.Join(err, cleanupErr)
	}
	return &App{Telemetry: telemetry, Config: cfg, reader: reader, worker: worker, done: make(chan struct{})}, nil
}

func (a *App) Start(ctx context.Context) error {
	defer close(a.done)
	a.Logger.Info(ctx, "Kafka share worker started", logger.FromPairs("kafka.topic", a.Config.Kafka.Topic, "kafka.group_id", a.Config.Kafka.GroupID, "worker.concurrency", a.Config.Concurrency))
	return a.worker.Run(ctx)
}

func (a *App) Shutdown(ctx context.Context) error {
	// Run joins all handlers before closing done. Only then may cleanup use the reader.
	select {
	case <-a.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return errors.Join(a.reader.Close(ctx), a.Telemetry.Shutdown(ctx))
}
