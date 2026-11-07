package observability

import (
	"context"
	"errors"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/twmb/franz-go/pkg/kgo"
)

type processingOutcome struct {
	operation string
	record    *kgo.Record
	duration  time.Duration
	err       error
	outcome   string
}

func newProcessingOutcome(operation string, record *kgo.Record, duration time.Duration, err error) processingOutcome {
	outcome := "success"
	switch {
	case errors.Is(err, middleware.ErrInvalidMessage):
		outcome = "invalid"
	case errors.Is(err, middleware.ErrHandlerPanic):
		outcome = "panic"
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "timeout"
	case errors.Is(err, context.Canceled):
		outcome = "canceled"
	case err != nil:
		outcome = "error"
	}
	return processingOutcome{operation: operation, record: record, duration: duration, err: err, outcome: outcome}
}
