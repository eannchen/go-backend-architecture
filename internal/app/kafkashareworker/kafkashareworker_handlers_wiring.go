package kafkashareworker

import (
	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/observability"
	recoverymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

func buildWorker(reader *kafkaclient.ShareReader, telemetry *appruntime.Telemetry, cfg config.KafkaShareWorkerConfig) (*kafkadelivery.NotificationShareWorker, error) {
	usecase := notification.New(telemetry.Logger)
	handler := kafkadelivery.NewNotificationHandler(usecase)
	// Listed outermost first. Context supplies trace metadata and the deadline;
	// observability records the outcome after recovery converts any handler panic.
	handler = middleware.Chain(handler,
		contextmw.New(telemetry.Observability.Tracer(), contextmw.Config{
			HandlingTimeout:      cfg.HandlingTimeout,
			AcknowledgmentMargin: cfg.AcknowledgmentMargin,
		}).Handler(),
		observabilitymw.New(telemetry.Observability.Tracer(), telemetry.Logger, telemetry.Observability.Meter()).Handler("notification.share.consume"),
		recoverymw.New(telemetry.Logger).Handler(),
	)
	return kafkadelivery.NewNotificationShareWorker(shareReaderAdapter{reader}, handler, kafkadelivery.ShareWorkerOptions{Concurrency: cfg.Concurrency})
}
