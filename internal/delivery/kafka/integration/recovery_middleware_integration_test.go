//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	notificationhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestRecoveredPanicDoesNotAdvanceProgress verifies middleware recovery reaches
// the worker's failure policy: ordinary progress stays uncommitted and a Share
// Group record is released for another attempt.
func TestRecoveredPanicDoesNotAdvanceProgress(t *testing.T) {
	for shared := range workflowFactories {
		name := "ordinary"
		if shared {
			name = "share"
		}
		t.Run(name, func(t *testing.T) {
			runtime := newTelemetry(t)
			topic := backend.CreateTopic(t, 1)
			group := topic + "-panic"
			prepareGroup(t, shared, group)
			log := newLog(func(_ context.Context, message string, _ ...logger.Fields) {
				if message == "notification request handled" {
					panic("private-data")
				}
			})
			handler := processingHandler(runtime, log, "notification.consume", notificationhandler.NewNotificationHandler(notification.New(log)))
			flow := newWorkflow(t, runtime, shared, topic, group, handler)
			writer := newPublisher(t, runtime, topic)
			ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
			defer cancel()
			running := startWorkers(t, middleware.ErrHandlerPanic, flow)
			if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte(notificationJSON)}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-running.done:
			case <-ctx.Done():
				t.Fatal("panic did not reach worker policy")
			}
			if len(log.ErrorCalls) != 1 {
				t.Fatal("missing recovery diagnostic")
			}
			reopened := newWorkflow(t, runtime, shared, topic, group, handler)
			record, err := reopened.next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if record.Offset != 0 {
				t.Fatal("panic advanced saved progress")
			}
			if shared && record.DeliveryCount() < 2 {
				t.Fatal("failed share delivery was not retried")
			}
			if err := reopened.complete(ctx, record); err != nil {
				t.Fatal(err)
			}
		})
	}
}
