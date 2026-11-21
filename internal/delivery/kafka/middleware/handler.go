package middleware

import (
	"context"
	"errors"
	"slices"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Handler processes one record. The worker decides how to save its outcome.
type Handler func(context.Context, *kgo.Record) error
type Middleware func(Handler) Handler

// ErrInvalidMessage identifies a message that cannot be decoded or validated.
// Each worker chooses whether to stop or reject it.
var ErrInvalidMessage = errors.New("invalid Kafka message")
var ErrHandlerPanic = errors.New("Kafka handler panicked")

// Chain runs middleware in the listed order, with the handler innermost.
func Chain(handler Handler, middleware ...Middleware) Handler {
	for _, m := range slices.Backward(middleware) {
		handler = m(handler)
	}
	return handler
}
