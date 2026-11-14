package notification

import (
	"context"
	"fmt"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/twmb/franz-go/pkg/kgo"
)

// NewNotificationHandler maps the Kafka payload to the transport-free usecase.
// Topic routing selects this handler; workers own the commit/acknowledgment policy.
func NewNotificationHandler(usecase notification.Usecase) middleware.Handler {
	return func(ctx context.Context, record *kgo.Record) error {
		request, err := decodeNotification(record.Value)
		if err != nil {
			return fmt.Errorf("%w: %w", middleware.ErrInvalidMessage, err)
		}
		if err := usecase.Handle(ctx, request); err != nil {
			return fmt.Errorf("handle notification: %w", err)
		}
		return nil
	}
}
