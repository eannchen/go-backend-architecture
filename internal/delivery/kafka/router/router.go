package router

import (
	"context"
	"fmt"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/twmb/franz-go/pkg/kgo"
)

// New binds subscribed topics to handlers at startup. The private map is read
// concurrently by share workers; later changes to the caller's map have no effect.
func New(topics []string, routes map[string]middleware.Handler) (middleware.Handler, error) {
	handlers := make(map[string]middleware.Handler, len(topics))
	for _, topic := range topics {
		handler := routes[topic]
		if handler == nil {
			return nil, fmt.Errorf("no Kafka handler registered for topic %q", topic)
		}
		handlers[topic] = handler
	}
	return func(ctx context.Context, record *kgo.Record) error {
		handler := handlers[record.Topic]
		if handler == nil {
			return fmt.Errorf("no Kafka handler registered for topic %q", record.Topic)
		}
		return handler(ctx, record)
	}, nil
}
