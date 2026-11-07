package kafkaworker

import (
	"github.com/eannchen/go-backend-architecture/internal/app/kafkaprocessing"
	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

func buildWorker(reader *kafkaclient.Reader, telemetry *appruntime.Telemetry, cfg config.KafkaWorkerConfig) *kafkadelivery.NotificationWorker {
	usecase := notification.New(telemetry.Logger)
	processing := kafkaprocessing.New(telemetry.Observability.Tracer(), telemetry.Logger, telemetry.Observability.Meter(), kafkaprocessing.Config{
		HandlingTimeout: cfg.HandlingTimeout,
	})
	handler := processing.Wrap("notification.consume", kafkadelivery.NewNotificationHandler(usecase))
	return kafkadelivery.NewNotificationWorker(reader, handler)
}
