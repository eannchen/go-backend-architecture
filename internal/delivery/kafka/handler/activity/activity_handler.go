package activity

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/usecase/activity"
)

// Processor is the application behavior required by this adapter.
type Processor interface {
	Handle(context.Context, activity.Request) error
}

func NewActivityHandler(usecase Processor) middleware.Handler {
	return func(ctx context.Context, record *kgo.Record) error {
		request, err := decodeActivity(record.Value)
		if err != nil {
			return fmt.Errorf("%w: %w", middleware.ErrInvalidMessage, err)
		}
		if err := usecase.Handle(ctx, request); err != nil {
			return fmt.Errorf("handle activity: %w", err)
		}
		return nil
	}
}
