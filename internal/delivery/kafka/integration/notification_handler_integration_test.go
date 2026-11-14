//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	notificationhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"

	"github.com/twmb/franz-go/pkg/kgo"
)

// notificationJSON is a wire example, independent of Go DTO serialization.
// Future AsyncAPI schema checks can validate this same message example.
const notificationJSON = `{"request_id":"req-1","recipient":"private-user","content":"private-content"}`

// TestNotificationHandlerWorkflow verifies real JSON delivery to the real
// usecase, trace continuity through kotel/middleware, and saved group progress.
func TestNotificationHandlerWorkflow(t *testing.T) {
	for shared := range workflowFactories {
		name := "ordinary"
		if shared {
			name = "share"
		}
		t.Run(name, func(t *testing.T) {
			runtime := newTelemetry(t)
			topic := backend.CreateTopic(t, 1)
			group := topic + "-notifications"
			prepareGroup(t, shared, group)
			ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
			defer cancel()
			published, span := runtime.Tracer().Start(ctx, "test", "publish-notification")
			defer span.Finish(nil)
			parent, valid := runtime.Tracer().TraceContext(published)
			if !valid {
				t.Fatal("publisher has no trace")
			}
			handled := make(chan string, 1)
			log := &loggertest.Logger{InfoFunc: func(ctx context.Context, message string, fields ...logger.Fields) {
				if message != "notification request handled" {
					return
				}
				trace, ok := runtime.Tracer().TraceContext(ctx)
				if !ok || trace.TraceID != parent.TraceID {
					handled <- "trace lost"
					return
				}
				values := logger.OptionalFields(fields...)
				if len(values) != 1 || values["notification.request_id"] != "req-1" {
					handled <- "wrong business result"
					return
				}
				handled <- "handled"
			}}
			handler := processingHandler(runtime, log, "notification.consume", notificationhandler.NewNotificationHandler(notification.New(log)))
			flow := newWorkflow(t, runtime, shared, topic, group, handler)
			writer := newPublisher(t, runtime, topic)
			running := startWorkers(t, nil, flow)
			if err := writer.WriteRecords(published, &kgo.Record{Context: published, Value: []byte(notificationJSON)}); err != nil {
				t.Fatal(err)
			}
			select {
			case result := <-handled:
				if result != "handled" {
					t.Fatal(result)
				}
			case <-ctx.Done():
				t.Fatal("notification not handled")
			}
			waitCheckpoint(t, flow)
			stopWorkers(t, running)
			// Reopen the same group and publish a marker. Reading offset one proves the
			// original record was committed/accepted, rather than merely not polled yet.
			reopened := newWorkflow(t, runtime, shared, topic, group, handler)

			if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte(notificationJSON)}); err != nil {
				t.Fatal(err)
			}
			record, err := reopened.next(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if record.Offset != 1 {
				t.Fatalf("saved notification replayed at offset %d", record.Offset)
			}
			if err := reopened.complete(ctx, record); err != nil {
				t.Fatal(err)
			}
		})
	}
}
