//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/consumer"
	notificationhandler "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/eannchen/go-backend-architecture/pkg/logger"
	"github.com/eannchen/go-backend-architecture/pkg/observability/otel"
)

func init() { workflowFactories[true] = newSharedWorkflow }

func newSharedWorkflow(t *testing.T, runtime *otel.Runtime, topic, group string, handler middleware.Handler) *workflow {
	t.Helper()
	checkpoints := make(chan struct{}, 16)
	hooks := otel.KafkaHooks(runtime.TracerProvider(), runtime.MeterProvider(), group)
	reader, err := kafkaclient.NewShareReader(config.KafkaShareConsumerConfig{
		Brokers: backend.Brokers, Topics: []string{topic}, GroupID: group, MaxPollRecords: 1,
		AcknowledgmentMode: config.KafkaShareAcknowledgmentExplicit,
		FetchMinBytes:      1, FetchMaxBytes: 1000000, FetchMaxWait: 100 * time.Millisecond, RequestTimeout: 5 * time.Second,
	}, nil, func(_ string, _ int32, err error) error { return err }, hooks...)
	if err != nil {
		t.Fatal(err)
	}
	observed := &observingShareReader{ShareReader: reader, checkpoints: checkpoints}
	delivery, err := consumer.NewShareConsumer(observed, handler, consumer.ShareConsumerOptions{Concurrency: 2})
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
			// Closing drains the queue before this test reopens the group.
			return reader.Close(ctx)
		},
	})
}

// TestShareConsumerRejectsMalformedMessage verifies the delivery policy against
// Kafka: malformed JSON ends delivery to this group, then valid work proceeds.
func TestShareConsumerRejectsMalformedMessage(t *testing.T) {
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

// TestShareConsumerRenewsRunningAndQueuedRecords verifies the actual consumer
// keeps both records locked beyond their original deadline while processing.
// A competing member must not take them over; later redelivery is allowed.
func TestShareConsumerRenewsRunningAndQueuedRecords(t *testing.T) {
	runtime := newTelemetry(t)
	topic := backend.CreateTopic(t, 1)
	group := topic + "-renew"
	backend.PrepareShareGroup(t, group, 3000)
	reader, err := kafkaclient.NewShareReader(config.KafkaShareConsumerConfig{
		Brokers: backend.Brokers, Topics: []string{topic}, GroupID: group,
		AcknowledgmentMode: config.KafkaShareAcknowledgmentExplicit, MaxPollRecords: 2,
		FetchMinBytes: 1, FetchMaxBytes: 1000000, FetchMaxWait: 100 * time.Millisecond, RequestTimeout: 5 * time.Second,
	}, nil, consumer.LogShareAcknowledgmentErrors(logger.NoopLogger{}, group))
	if err != nil {
		t.Fatal(err)
	}
	checkpoints := make(chan struct{}, 4)
	observed := &renewalObservingReader{
		observingShareReader: &observingShareReader{ShareReader: reader, checkpoints: checkpoints},
		renewed:              make(chan struct{}, 16),
	}
	started := make(chan *kgo.Record, 2)
	finish := make(chan struct{})
	release := sync.OnceFunc(func() { close(finish) })
	defer release()
	handler := contextmw.New(runtime.Tracer(), contextmw.Config{
		HandlingTimeout: 15 * time.Second, AcknowledgmentMargin: 100 * time.Millisecond, RenewalEnabled: true,
	}).Handler()(func(ctx context.Context, record *kgo.Record) error {
		started <- record
		if record.Offset == 0 {
			select {
			case <-finish:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	delivery, err := consumer.NewShareConsumer(observed, handler, consumer.ShareConsumerOptions{
		Concurrency: 1, RenewalInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	flow := ownWorkflow(t, &workflow{
		task: appruntime.Worker{Name: topic, Run: delivery.Run, Shutdown: reader.Close}, checkpoints: checkpoints,
	})
	writer := newPublisher(t, runtime, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("slow")}, &kgo.Record{Value: []byte("queued")}); err != nil {
		t.Fatal(err)
	}
	running := startWorkers(t, nil, flow)
	var first *kgo.Record
	select {
	case first = <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Observe renewal requests until the original deadline has passed. The
	// competing consumer below checks the broker outcome, not just these events.
	for time.Now().Before(first.AcquisitionDeadline().Add(500 * time.Millisecond)) {
		select {
		case <-observed.renewed:
		case <-running.done:
			t.Fatalf("renewing consumer stopped: %v", running.err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	competitor := newWorkflow(t, runtime, true, topic, group, func(context.Context, *kgo.Record) error { return nil })
	idle, stop := context.WithTimeout(ctx, 1500*time.Millisecond)
	_, err = competitor.next(idle)
	stop()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("another member acquired renewed work: %v", err)
	}
	release()
	select {
	case second := <-started:
		if second.Offset != 1 || second.DeliveryCount() != 1 || first.DeliveryCount() != 1 {
			t.Fatal("renewal consumed a delivery attempt or lost queued work")
		}
	case <-running.done:
		t.Fatalf("queued handler failed: %v", running.err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	waitCheckpoint(t, flow)
	stopWorkers(t, running)
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("new")}); err != nil {
		t.Fatal(err)
	}
	// At-least-once delivery allows replay if an asynchronous final ack failed.
	// Accept such replays and verify that the group still reaches the new record.
	for range 3 {
		next, err := competitor.next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := competitor.complete(ctx, next); err != nil {
			t.Fatal(err)
		}
		if next.Offset == 2 {
			return
		}
		competitor = newWorkflow(t, runtime, true, topic, group, func(context.Context, *kgo.Record) error { return nil })
	}
	t.Fatal("group did not advance after handling possible redelivery")
}

// renewalObservingReader signals queued requests. Only the competing consumer
// and final broker outcomes verify whether the records remained acquired.
type renewalObservingReader struct {
	*observingShareReader
	renewed chan struct{}
}

func (r *renewalObservingReader) RenewRemaining() error {
	if err := r.ShareReader.RenewRemaining(); err != nil {
		return err
	}
	select {
	case r.renewed <- struct{}{}:
	default:
	}
	return nil
}
