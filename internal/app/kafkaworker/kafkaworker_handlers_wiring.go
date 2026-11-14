package kafkaworker

import (
	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	activityhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/activity"
	notificationhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/observability"
	recoverymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/router"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/worker"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/usecase/activity"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

func buildWorker(reader *kafkaclient.Reader, telemetry *appruntime.Telemetry, cfg config.KafkaWorkerReaderConfig, routes config.KafkaWorkerRoutesConfig) (*worker.Worker, error) {
	// Apply the same middleware order to each route, with a distinct operation
	// name so logs, traces, and metrics identify the selected workflow.
	decorate := func(handler middleware.Handler, operation string) middleware.Handler {
		return middleware.Chain(handler,
			contextmw.New(telemetry.Observability.Tracer(), contextmw.Config{
				HandlingTimeout: cfg.HandlingTimeout,
			}).Handler(),
			observabilitymw.New(telemetry.Observability.Tracer(), telemetry.Logger, telemetry.Observability.Meter()).Handler(operation),
			recoverymw.New(telemetry.Logger).Handler(),
		)
	}
	handler, err := router.New(cfg.Kafka.Topics, map[string]middleware.Handler{
		routes.NotificationTopic: decorate(notificationhandler.NewNotificationHandler(notification.New(telemetry.Logger)), "notification.consume"),
		routes.ActivityTopic:     decorate(activityhandler.NewActivityHandler(activity.New(telemetry.Logger)), "activity.consume"),
	})
	if err != nil {
		return nil, err
	}
	return worker.NewWorker(reader, handler), nil
}
