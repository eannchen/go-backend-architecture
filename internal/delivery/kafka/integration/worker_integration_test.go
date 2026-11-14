//go:build integration

package integration

import (
	"context"
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

func init() { workflowFactories[false] = newTraditionalWorkflow }

func newTraditionalWorkflow(t *testing.T, runtime *otelinfra.Runtime, topic, group string, handler middleware.Handler) *workflow {
	t.Helper()
	checkpoints := make(chan struct{}, 16)
	hooks := otelinfra.KafkaHooks(runtime.TracerProvider(), runtime.MeterProvider(), group)

	reader, err := kafkaclient.NewReader(config.KafkaConsumerConfig{
		Brokers: backend.Brokers, Topics: []string{topic}, GroupID: group, InitialOffset: config.KafkaInitialEarliest,
		FetchMinBytes: 1, FetchMaxBytes: 1000000, FetchMaxWait: 100 * time.Millisecond, RequestTimeout: 5 * time.Second,
		RebalanceTimeout: time.Minute, MaxConcurrentFetches: 1,
	}, nil, hooks...)
	if err != nil {
		t.Fatal(err)
	}
	delivery := worker.NewWorker(&observingReader{Reader: reader, checkpoints: checkpoints}, handler)
	return ownWorkflow(t, &workflow{
		task:        appruntime.Worker{Name: topic, Run: delivery.Run, Shutdown: reader.Close},
		checkpoints: checkpoints, next: reader.FetchRecord,
		complete: func(ctx context.Context, record *kgo.Record) error {
			defer reader.AllowRebalance()
			return reader.CommitRecords(ctx, record)
		},
	})
}

// TestWorkerMalformedMessageRemainsUncommitted verifies the ordinary delivery
// failure policy through a real broker and handler, including replay on restart.
func TestWorkerMalformedMessageRemainsUncommitted(t *testing.T) {
	runtime := newTelemetry(t)
	topic := backend.CreateTopic(t, 1)
	group := topic + "-invalid"
	prepareGroup(t, false, group)
	log := newLog(func(context.Context, string, ...logger.Fields) {})
	handler := processingHandler(runtime, log, "notification.consume", notificationhandler.NewNotificationHandler(notification.New(log)))
	flow := newWorkflow(t, runtime, false, topic, group, handler)
	writer := newPublisher(t, runtime, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	running := startWorkers(t, middleware.ErrInvalidMessage, flow)
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-running.done:
	case <-ctx.Done():
		t.Fatal("invalid message did not stop the worker")
	}
	if len(log.WarnCalls) != 1 {
		t.Fatal("invalid message was not reported")
	}
	for _, call := range log.InfoCalls {
		if call.Message == "notification request handled" {
			t.Fatal("invalid message reached usecase")
		}
	}
	reopened := newWorkflow(t, runtime, false, topic, group, handler)
	record, err := reopened.next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if record.Offset != 0 {
		t.Fatal("invalid message was committed")
	}
	if err := reopened.complete(ctx, record); err != nil {
		t.Fatal(err)
	}
}
