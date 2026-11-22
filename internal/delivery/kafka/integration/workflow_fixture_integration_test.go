//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/observability"
	recoverymw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/recovery"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka/kafkatest"
	"github.com/eannchen/go-backend-architecture/pkg/logger"
	"github.com/eannchen/go-backend-architecture/pkg/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/pkg/observability/otel"
)

var backend *kafkatest.Broker

// TestMain owns one real broker for the delivery package. Tests own their topics,
// groups, clients, and telemetry; a missing backend fails rather than skips.
func TestMain(m *testing.M) {
	os.Exit(kafkatest.RunSuite(m, func(b *kafkatest.Broker) { backend = b }))
}

func newTelemetry(t *testing.T) *otel.Runtime {
	t.Helper()
	runtime, err := otel.Setup(t.Context(), otel.Config{TraceSamplingRatio: 1}, "kafka-delivery-test", "test")
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

func processingHandler(runtime *otel.Runtime, log logger.Logger, operation string, handler middleware.Handler) middleware.Handler {
	return middleware.Chain(handler,
		contextmw.New(runtime.Tracer(), contextmw.Config{HandlingTimeout: time.Minute, AcknowledgmentMargin: time.Second}).Handler(),
		observabilitymw.New(runtime.Tracer(), log, runtime.Meter()).Handler(operation),
		recoverymw.New(log).Handler(),
	)
}

// Checkpoints observe the next fetch after processing. Ordinary commits have
// finished; Share Group decisions may still be pending. Reader shutdown drains
// those acknowledgments before tests reopen the group.
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

// newWorkflow builds real clients and consumers. Group preparation is owned by
// the test, allowing the same group to be reopened to inspect saved progress.
func newWorkflow(t *testing.T, runtime *otel.Runtime, shared bool, topic, group string, handler middleware.Handler) *workflow {
	t.Helper()
	factory := workflowFactories[shared]
	if factory == nil {
		t.Fatal("consumer type is not installed")
	}
	return factory(t, runtime, topic, group, handler)
}

type workflowFactory func(*testing.T, *otel.Runtime, string, string, middleware.Handler) *workflow

// Each consumer-specific test file registers its factory. Profile selection removes
// that file with its production consumer, so shared scenarios test installed styles.
var workflowFactories = map[bool]workflowFactory{}

func prepareGroup(t *testing.T, shared bool, group string) {
	t.Helper()
	if shared {
		backend.PrepareShareGroup(t, group, 30000)
	} else {
		backend.CleanupGroup(t, group)
	}
}

func newPublisher(t *testing.T, runtime *otel.Runtime, topic string) *kafkaclient.Writer {
	t.Helper()
	writer, err := kafkaclient.NewWriter(backend.PublisherConfig(topic), nil, otel.KafkaHooks(runtime.TracerProvider(), runtime.MeterProvider(), "")...)
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
		running.err = (appruntime.WorkerGroup{Logger: logger.NoopLogger{}, Workers: tasks, StopOnError: true, ShutdownTimeout: 10 * time.Second}).Run(ctx)
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
