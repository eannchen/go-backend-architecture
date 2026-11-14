//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	activityhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/activity"
	notificationhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/router"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/activity"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"

	"github.com/twmb/franz-go/pkg/kgo"
)

const activityJSON = `{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}`

// TestActivityHandlerDoesNotBlockNotifications holds the real activity usecase's
// logging boundary open while a second reader handles a notification. Both
// worker styles must route their own payload and save progress independently.
func TestActivityHandlerDoesNotBlockNotifications(t *testing.T) {
	for shared := range workflowFactories {
		name := "ordinary"
		if shared {
			name = "share"
		}
		t.Run(name, func(t *testing.T) {
			runtime := newTelemetry(t)
			mailTopic, activityTopic := backend.CreateTopic(t, 1), backend.CreateTopic(t, 1)
			mailGroup, activityGroup := mailTopic+"-mail", activityTopic+"-activity"
			prepareGroup(t, shared, mailGroup)
			prepareGroup(t, shared, activityGroup)
			ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
			defer cancel()
			activityStarted := make(chan string, 1)
			release := make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer unblock()
			mailHandled := make(chan string, 1)
			activityLog := &loggertest.Logger{InfoFunc: func(ctx context.Context, message string, fields ...logger.Fields) {
				if message != "activity event handled" {
					return
				}
				id, _ := logger.OptionalFields(fields...)["activity.event_id"].(string)
				activityStarted <- id
				select {
				case <-release:
				case <-ctx.Done():
				}
			}}
			mailLog := &loggertest.Logger{InfoFunc: func(_ context.Context, message string, fields ...logger.Fields) {
				if message == "notification request handled" {
					id, _ := logger.OptionalFields(fields...)["notification.request_id"].(string)
					mailHandled <- id
				}
			}}
			routes, err := router.New([]string{mailTopic, activityTopic}, map[string]middleware.Handler{
				mailTopic:     processingHandler(runtime, mailLog, "notification.consume", notificationhandler.NewNotificationHandler(notification.New(mailLog))),
				activityTopic: processingHandler(runtime, activityLog, "activity.consume", activityhandler.NewActivityHandler(activity.New(activityLog))),
			})
			if err != nil {
				t.Fatal(err)
			}
			mailFlow := newWorkflow(t, runtime, shared, mailTopic, mailGroup, routes)
			activityFlow := newWorkflow(t, runtime, shared, activityTopic, activityGroup, routes)
			mailWriter, activityWriter := newPublisher(t, runtime, mailTopic), newPublisher(t, runtime, activityTopic)
			running := startWorkers(t, nil, mailFlow, activityFlow)
			if err := activityWriter.WriteRecords(ctx, &kgo.Record{Value: []byte(activityJSON)}); err != nil {
				t.Fatal(err)
			}
			select {
			case id := <-activityStarted:
				if id != "evt-1" {
					t.Fatal("wrong activity request")
				}
			case <-ctx.Done():
				t.Fatal("activity not started")
			}
			if err := mailWriter.WriteRecords(ctx, &kgo.Record{Value: []byte(notificationJSON)}); err != nil {
				t.Fatal(err)
			}
			select {
			case id := <-mailHandled:
				if id != "req-1" {
					t.Fatal("wrong notification request")
				}
			case <-ctx.Done():
				t.Fatal("notification blocked by activity")
			}
			waitCheckpoint(t, mailFlow)
			// Release only after the independent notification committed/acknowledged.
			// The deferred release also unblocks the handler after a failed assertion.
			unblock()
			waitCheckpoint(t, activityFlow)
			stopWorkers(t, running)
		})
	}
}
