package kafkashareworker

import (
	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	"github.com/eannchen/go-backend-architecture/internal/app/wiring/kafkaprocessing"
	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

func buildWorker(reader *kafkaclient.ShareReader, telemetry *appruntime.Telemetry, cfg config.KafkaShareWorkerConfig) (*kafkadelivery.NotificationShareWorker, error) {
	processing := kafkaprocessing.New(telemetry.Observability.Tracer(), telemetry.Logger, telemetry.Observability.Meter(), kafkaprocessing.Config{
		HandlingTimeout:      cfg.HandlingTimeout,
		AcknowledgmentMargin: cfg.AcknowledgmentMargin,
	})
	handler := processing.Wrap("notification.share.consume", kafkadelivery.NewNotificationHandler(notification.New(telemetry.Logger)))
	return kafkadelivery.NewNotificationShareWorker(shareReaderAdapter{reader}, handler, kafkadelivery.ShareWorkerOptions{Concurrency: cfg.Concurrency})
}
