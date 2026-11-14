//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	notificationhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/worker"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	otelinfra "github.com/eannchen/go-backend-architecture/internal/infra/observability/otel"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"

	"github.com/twmb/franz-go/pkg/kgo"
)

func init() { workflowFactories[true] = newSharedWorkflow }

func newSharedWorkflow(t *testing.T, runtime *otelinfra.Runtime, topic, group string, handler middleware.Handler) *workflow {
	t.Helper()
	checkpoints := make(chan struct{}, 16)
	hooks := otelinfra.KafkaHooks(runtime.TracerProvider(), runtime.MeterProvider(), group)
	reader, err := kafkaclient.NewShareReader(config.KafkaShareConsumerConfig{
		Brokers: backend.Brokers, Topics: []string{topic}, GroupID: group, MaxPollRecords: 1,
		FetchMinBytes: 1, FetchMaxBytes: 1000000, FetchMaxWait: 100 * time.Millisecond, RequestTimeout: 5 * time.Second,
	}, nil, hooks...)
	if err != nil {
		t.Fatal(err)
	}
	observed := &observingShareReader{ShareReader: reader, checkpoints: checkpoints}
	delivery, err := worker.NewShareWorker(observed, handler, worker.ShareWorkerOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	return ownWorkflow(t, &workflow{
		task:        appruntime.Worker{Name: topic, Run: delivery.Run, Shutdown: reader.Close},
		checkpoints: checkpoints,
		next: func(ctx context.Context) (*kgo.Record, error) {
			records, err := reader.FetchRecords(ctx)
			if err != nil {
				return nil, err
			}
			if len(records) != 1 {
				return nil, errors.New("expected one acquired record")
			}
			return records[0], nil
		},
		complete: func(ctx context.Context, record *kgo.Record) error {
			if err := reader.Accept(record); err != nil {
				return err
			}
			return reader.FlushAcknowledgments(ctx)
		},
	})
}

// TestShareWorkerRejectsMalformedMessage verifies the delivery policy against
// Kafka: malformed JSON ends delivery to this group, then valid work proceeds.
func TestShareWorkerRejectsMalformedMessage(t *testing.T) {
	runtime := newTelemetry(t)
	topic := backend.CreateTopic(t, 1)
	group := topic + "-reject"
	prepareGroup(t, true, group)
	log := newLog(func(context.Context, string, ...logger.Fields) {})
	handler := processingHandler(runtime, log, "notification.consume", notificationhandler.NewNotificationHandler(notification.New(log)))
	flow := newWorkflow(t, runtime, true, topic, group, handler)
	writer := newPublisher(t, runtime, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	running := startWorkers(t, nil, flow)
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("{}")}, &kgo.Record{Value: []byte(notificationJSON)}); err != nil {
		t.Fatal(err)
	}
	waitCheckpoint(t, flow)
	waitCheckpoint(t, flow)
	stopWorkers(t, running)
	handled := 0
	for _, call := range log.InfoCalls {
		if call.Message == "notification request handled" {
			handled++
		}
	}
	if handled != 1 || len(log.WarnCalls) != 1 {
		t.Fatal("malformed message reached the workflow or valid work was blocked")
	}
	reopened := newWorkflow(t, runtime, true, topic, group, handler)
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte(notificationJSON)}); err != nil {
		t.Fatal(err)
	}
	record, err := reopened.next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if record.Offset != 2 {
		t.Fatalf("terminal outcomes replayed at offset %d", record.Offset)
	}
	if err := reopened.complete(ctx, record); err != nil {
		t.Fatal(err)
	}
}
