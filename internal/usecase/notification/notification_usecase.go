package notification

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Request is application input. JSON fields and Kafka message metadata belong
// to the consumer adapter, so this type can also be used by another transport.
type Request struct {
	RequestID string
	Recipient string
	Content   string
}

// Usecase implements the notification workflow.
type Usecase struct{ log logger.Logger }

func New(log logger.Logger) *Usecase {
	return &Usecase{log: log}
}

func (u *Usecase) Handle(ctx context.Context, request Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// This template simulates handling; it does not send an email or text message.
	// Keep recipient and content out of logs. Request IDs allow correlation without
	// exposing the notification payload. A replay can produce this log again.
	u.log.Info(ctx, "notification request handled", logger.FromPairs("notification.request_id", request.RequestID))
	return nil
}
