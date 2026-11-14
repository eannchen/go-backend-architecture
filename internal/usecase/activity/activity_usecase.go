package activity

import (
	"context"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Request represents an activity independently of its incoming transport.
type Request struct {
	EventID    string
	Action     string
	OccurredAt time.Time
}

type Usecase interface {
	Handle(context.Context, Request) error
}

type impl struct{ log logger.Logger }

func New(log logger.Logger) Usecase {
	return &impl{log: log}
}

func (u *impl) Handle(ctx context.Context, request Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Simulate activity processing. Only the event ID is logged because the
	// action can describe private user behavior. Replays can produce this log again.
	u.log.Info(ctx, "activity event handled", logger.FromPairs("activity.event_id", request.EventID))
	return nil
}
