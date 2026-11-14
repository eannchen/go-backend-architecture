//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/observability"
	recoverymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka/kafkatest"
	otelinfra "github.com/eannchen/go-backend-architecture/internal/infra/observability/otel"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"

	"github.com/twmb/franz-go/pkg/kgo"
)

var backend *kafkatest.Broker

// TestMain owns one real broker for the delivery package. Tests own their topics,
// groups, clients, and telemetry; a missing backend fails rather than skips.
func TestMain(m *testing.M) {
	os.Exit(kafkatest.RunSuite(m, func(b *kafkatest.Broker) { backend = b }))
}

func newTelemetry(t *testing.T) *otelinfra.Runtime {
	t.Helper()
	runtime, err := otelinfra.Setup(t.Context(), config.OTelConfig{TraceSamplingRatio: 1}, "kafka-delivery-test", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := runtime.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return runtime
}

func processingHandler(runtime *otelinfra.Runtime, log logger.Logger, operation string, handler middleware.Handler) middleware.Handler {
	return middleware.Chain(handler,
		contextmw.New(runtime.Tracer(), contextmw.Config{HandlingTimeout: time.Minute, AcknowledgmentMargin: time.Second}).Handler(),
		observabilitymw.New(runtime.Tracer(), log, runtime.Meter()).Handler(operation),
		recoverymw.New(log).Handler(),
	)
}

// Checkpoints observe the next worker fetch, after the preceding commit or
// acknowledgment flush. Every reader operation still uses the real client.
type observingReader struct {
	*kafkaclient.Reader
	checkpoints chan struct{}
	polled      bool
}

func (r *observingReader) FetchRecord(ctx context.Context) (*kgo.Record, error) {
	if r.polled {
		select {
		case r.checkpoints <- struct{}{}:
		default:
		}
	}
	r.polled = true
	return r.Reader.FetchRecord(ctx)
}

type observingShareReader struct {
	*kafkaclient.ShareReader
	checkpoints chan struct{}
	polled      bool
}

func (r *observingShareReader) FetchRecords(ctx context.Context) ([]*kgo.Record, error) {
	if r.polled {
		select {
		case r.checkpoints <- struct{}{}:
		default:
		}
	}
	r.polled = true
	return r.ShareReader.FetchRecords(ctx)
}

type workflow struct {
	managed     bool
	task        appruntime.Worker
	checkpoints <-chan struct{}
	next        func(context.Context) (*kgo.Record, error)
	complete    func(context.Context, *kgo.Record) error
}

// newWorkflow builds real clients and workers. Group preparation is owned by
// the test, allowing the same group to be reopened to inspect saved progress.
func newWorkflow(t *testing.T, runtime *otelinfra.Runtime, shared bool, topic, group string, handler middleware.Handler) *workflow {
	t.Helper()
	factory := workflowFactories[shared]
	if factory == nil {
		t.Fatal("worker style is not installed")
	}
	return factory(t, runtime, topic, group, handler)
}

type workflowFactory func(*testing.T, *otelinfra.Runtime, string, string, middleware.Handler) *workflow

// Each worker-specific test file registers its factory. Profile selection removes
// that file with its production worker, so shared scenarios test installed styles.
var workflowFactories = map[bool]workflowFactory{}

func prepareGroup(t *testing.T, shared bool, group string) {
	t.Helper()
	if shared {
		backend.PrepareShareGroup(t, group, 30000)
	} else {
		backend.CleanupGroup(t, group)
	}
}

func newPublisher(t *testing.T, runtime *otelinfra.Runtime, topic string) *kafkaclient.Writer {
	t.Helper()
	writer, err := kafkaclient.NewWriter(config.KafkaPublisherConfig{
		Brokers: backend.Brokers, Topic: topic, PartitionStrategy: config.PartitionRoundRobin, RequiredAcks: config.KafkaAcksAll,
		RequestTimeout: 5 * time.Second, ProduceTimeout: 5 * time.Second, DeliveryTimeout: 10 * time.Second,
		MaxBufferedRecords: 25, BatchBytes: 65536, BatchTimeout: time.Millisecond, MaxAttempts: 3,
		WriteBackoffMin: 10 * time.Millisecond, WriteBackoffMax: 100 * time.Millisecond, Compression: config.CompressionNone,
	}, nil, otelinfra.KafkaHooks(runtime.TracerProvider(), runtime.MeterProvider(), "")...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	return writer
}

type runningWorkers struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

// startWorkers owns cancellation and joining even if an assertion fails early.
func startWorkers(t *testing.T, want error, flows ...*workflow) *runningWorkers {
	t.Helper()
	tasks := make([]appruntime.Worker, 0, len(flows))
	for _, flow := range flows {
		flow.managed = true
		tasks = append(tasks, flow.task)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	running := &runningWorkers{cancel: cancel, done: make(chan struct{})}
	go func() {
		running.err = (appruntime.WorkerGroup{Workers: tasks, StopOnError: true, ShutdownTimeout: 10 * time.Second}).Run(ctx)
		close(running.done)
	}()
	t.Cleanup(func() {
		running.cancel()
		select {
		case <-running.done:
			if !errors.Is(running.err, want) {
				t.Errorf("worker result = %v, want %v", running.err, want)
			}
		case <-time.After(15 * time.Second):
			t.Error("workers did not stop")
		}
	})
	return running
}
func waitCheckpoint(t *testing.T, flow *workflow) {
	t.Helper()
	select {
	case <-flow.checkpoints:
	case <-time.After(30 * time.Second):
		t.Fatal("processing did not commit/acknowledge")
	}
}
func stopWorkers(t *testing.T, running *runningWorkers) {
	t.Helper()
	running.cancel()
	select {
	case <-running.done:
	case <-time.After(15 * time.Second):
		t.Fatal("workers did not stop")
	}
}

// ownWorkflow closes clients built before a test starts its processing loop.
// Once startWorkers takes ownership, the coordinator performs reader cleanup.
func ownWorkflow(t *testing.T, flow *workflow) *workflow {
	t.Helper()
	t.Cleanup(func() {
		if flow.managed {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := flow.task.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return flow
}

func newLog(info func(context.Context, string, ...logger.Fields)) *loggertest.Logger {
	return &loggertest.Logger{
		InfoFunc:         info,
		WarnFunc:         func(context.Context, string, ...logger.Fields) {},
		ErrorFunc:        func(context.Context, string, error, ...logger.Fields) {},
		ErrorNoStackFunc: func(context.Context, string, error, ...logger.Fields) {},
	}
}
