//go:build integration

package kafkashareworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka/kafkatest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification/notificationtest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var brokers []string
var testSequence atomic.Int64

// TestMain owns one disposable Kafka broker for all composition tests.
func TestMain(m *testing.M) { os.Exit(runIntegration(m)) }
func runIntegration(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, addresses, err := kafkatest.StartBroker(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if container != nil {
			if err := testcontainers.TerminateContainer(container); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		return 1
	}
	brokers = addresses
	code := m.Run()
	if err := testcontainers.TerminateContainer(container); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	return code
}

// TestShareWorkersHandleOnePartitionConcurrently uses the actual delivery worker
// and app adapter to prove two processes' worth of pools can share one partition.
func TestShareWorkersHandleOnePartitionConcurrently(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	cfg, admin := integrationSubscription(t, ctx)
	for i := 0; i < 4; i++ {
		produce(t, ctx, admin, cfg.Topic, fmt.Sprintf(`{"request_id":"req-%d","recipient":"user","content":"ready"}`, i))
	}
	started := make(chan string, 4)
	release := make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	var active, peak atomic.Int32
	usecase := &notificationtest.Usecase{HandleFunc: func(ctx context.Context, r notification.Request) error {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		started <- r.RequestID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	// Capture successful broker-confirmed flushes before stopping either worker.
	flushed := make(chan struct{}, 4)
	results := make(chan error, 2)
	var owners sync.WaitGroup
	workerCtx, stopWorkers := context.WithCancel(ctx)
	for range 2 {
		reader := integrationReader(t, cfg)
		bridge := observedReader{shareReaderAdapter: shareReaderAdapter{reader}, flushed: flushed}
		worker, err := kafkadelivery.NewNotificationShareWorker(bridge, usecase, nil, workerOptions())
		if err != nil {
			t.Fatal(err)
		}
		owners.Go(func() { results <- worker.Run(workerCtx) })
	}
	t.Cleanup(func() { stopWorkers(); unblock.Do(func() { close(release) }); owners.Wait() })
	seen := map[string]bool{}
	for range 4 {
		select {
		case id := <-started:
			if seen[id] {
				t.Fatalf("duplicate acquired work: %s", id)
			}
			seen[id] = true
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if peak.Load() != 4 {
		t.Fatalf("peak=%d want=4 across two pools", peak.Load())
	}
	unblock.Do(func() { close(release) })
	for range 2 {
		select {
		case <-flushed:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	stopWorkers()
	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	owners.Wait()
	// Reader cleanup happens after owners join (LIFO cleanup registration).
}

type observedReader struct {
	shareReaderAdapter
	flushed chan<- struct{}
}

func (r observedReader) FlushAcknowledgments(ctx context.Context) error {
	if err := r.shareReaderAdapter.FlushAcknowledgments(ctx); err != nil {
		return err
	}
	r.flushed <- struct{}{}
	return nil
}

// TestShareWorkerRestartPreservesIndividualOutcomes runs delivery through the
// actual client: accepted/rejected records stay done; failed records return.
func TestShareWorkerRestartPreservesIndividualOutcomes(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	cfg, admin := integrationSubscription(t, ctx)
	cfg.MaxPollRecords = 1
	produce(t, ctx, admin, cfg.Topic, `{"request_id":"good","recipient":"user","content":"ready"}`)
	produce(t, ctx, admin, cfg.Topic, `invalid payload`)
	produce(t, ctx, admin, cfg.Topic, `{"request_id":"retry","recipient":"user","content":"ready"}`)
	reader := integrationReader(t, cfg)
	failure := errors.New("temporary downstream failure")
	usecase := &notificationtest.Usecase{HandleFunc: func(_ context.Context, r notification.Request) error {
		if r.RequestID == "retry" {
			return failure
		}
		return nil
	}}
	worker, err := kafkadelivery.NewNotificationShareWorker(shareReaderAdapter{reader}, usecase, nil, workerOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Run(ctx); !errors.Is(err, failure) {
		t.Fatalf("worker error=%v", err)
	}
	closeReader(t, reader)
	next := integrationReader(t, cfg)
	batch, err := next.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	records := batch.Records()
	if len(records) != 1 || records[0].Offset != 2 || records[0].DeliveryCount() < 2 {
		t.Fatalf("unexpected restart records=%+v", records)
	}
	if err := batch.Mark(records[0], kafkaclient.ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := next.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
	// Poll with a short deadline to ensure neither successful nor rejected records return.
	emptyCtx, stop := context.WithTimeout(ctx, 500*time.Millisecond)
	defer stop()
	if _, err := next.FetchBatch(emptyCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected no remaining work: %v", err)
	}
}

// TestShareWorkerLockDeadlinePreventsSlowSuccess verifies the SDK's actual lock
// deadline bounds handling even when the configured handler timeout is longer.
func TestShareWorkerLockDeadlinePreventsSlowSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	cfg, admin := integrationSubscription(t, ctx)
	setLockDuration(t, ctx, admin, cfg.GroupID, "5000")
	produce(t, ctx, admin, cfg.Topic, `{"request_id":"slow","recipient":"user","content":"ready"}`)
	reader := integrationReader(t, cfg)
	var observedDeadline time.Time
	var acquisitionDeadline time.Time
	bridge := deadlineReader{shareReaderAdapter: shareReaderAdapter{reader}, deadline: &acquisitionDeadline}
	usecase := &notificationtest.Usecase{HandleFunc: func(ctx context.Context, _ notification.Request) error {
		observedDeadline, _ = ctx.Deadline()
		<-ctx.Done()
		return ctx.Err()
	}}
	options := workerOptions()
	options.HandlingTimeout = time.Minute
	options.AcknowledgmentMargin = 2 * time.Second
	worker, err := kafkadelivery.NewNotificationShareWorker(bridge, usecase, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
	if observedDeadline.IsZero() || !observedDeadline.Equal(acquisitionDeadline.Add(-options.AcknowledgmentMargin)) {
		t.Fatalf("handler deadline=%v lock=%v", observedDeadline, acquisitionDeadline)
	}
	closeReader(t, reader)
	next := integrationReader(t, cfg)
	batch, err := next.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Records()[0].DeliveryCount() < 2 {
		t.Fatal("timed-out record was not released")
	}
}

type deadlineReader struct {
	shareReaderAdapter
	deadline *time.Time
}

func (r deadlineReader) FetchBatch(ctx context.Context) (kafkadelivery.ShareWorkBatch, error) {
	batch, err := r.shareReaderAdapter.FetchBatch(ctx)
	if err == nil {
		*r.deadline = batch.Records()[0].AcquisitionDeadline()
	}
	return batch, err
}

func workerOptions() kafkadelivery.ShareWorkerOptions {
	return kafkadelivery.ShareWorkerOptions{Concurrency: 2, HandlingTimeout: 10 * time.Second, AcknowledgmentMargin: 3 * time.Second}
}
func integrationReader(t *testing.T, cfg config.KafkaShareConsumerConfig) *kafkaclient.ShareReader {
	t.Helper()
	reader, err := kafkaclient.NewShareReader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	return reader
}
func closeReader(t *testing.T, reader *kafkaclient.ShareReader) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := reader.Close(ctx); err != nil {
		t.Error(err)
	}
}
func produce(t *testing.T, ctx context.Context, client *kgo.Client, topic, payload string) {
	t.Helper()
	if err := client.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: []byte(payload)}).FirstErr(); err != nil {
		t.Fatal(err)
	}
}
func setLockDuration(t *testing.T, ctx context.Context, client *kgo.Client, group, duration string) {
	t.Helper()
	req := kmsg.NewPtrIncrementalAlterConfigsRequest()
	resource := kmsg.NewIncrementalAlterConfigsRequestResource()
	resource.ResourceType = kmsg.ConfigResourceTypeGroupConfig
	resource.ResourceName = group
	for key, value := range map[string]string{"share.auto.offset.reset": "earliest", "share.record.lock.duration.ms": duration} {
		entry := kmsg.NewIncrementalAlterConfigsRequestResourceConfig()
		entry.Name = key
		entry.Value = kmsg.StringPtr(value)
		resource.Configs = append(resource.Configs, entry)
	}
	req.Resources = []kmsg.IncrementalAlterConfigsRequestResource{resource}
	response, err := client.Request(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range response.(*kmsg.IncrementalAlterConfigsResponse).Resources {
		if err := kerr.ErrorForCode(r.ErrorCode); err != nil {
			t.Fatal(err)
		}
	}
}
func integrationSubscription(t *testing.T, ctx context.Context) (config.KafkaShareConsumerConfig, *kgo.Client) {
	t.Helper()
	topic := fmt.Sprintf("share-worker-%d", testSequence.Add(1))
	group := topic + "-group"
	admin, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	req := kmsg.NewPtrCreateTopicsRequest()
	req.TimeoutMillis = 10000
	item := kmsg.NewCreateTopicsRequestTopic()
	item.Topic = topic
	item.NumPartitions = 1
	item.ReplicationFactor = 1
	req.Topics = []kmsg.CreateTopicsRequestTopic{item}
	response, err := admin.Request(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range response.(*kmsg.CreateTopicsResponse).Topics {
		if err := kerr.ErrorForCode(r.ErrorCode); err != nil {
			t.Fatal(err)
		}
	}
	setLockDuration(t, ctx, admin, group, "30000")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// All reader cleanups run first. Delete each test's group, config, and topic.
		deleteGroup := kmsg.NewPtrDeleteGroupsRequest()
		deleteGroup.Groups = []string{group}
		response, err := admin.Request(ctx, deleteGroup)
		if err != nil {
			t.Error(err)
		} else {
			for _, r := range response.(*kmsg.DeleteGroupsResponse).Groups {
				if err := kerr.ErrorForCode(r.ErrorCode); err != nil {
					t.Error(err)
				}
			}
		}
		reset := kmsg.NewPtrIncrementalAlterConfigsRequest()
		resource := kmsg.NewIncrementalAlterConfigsRequestResource()
		resource.ResourceType = kmsg.ConfigResourceTypeGroupConfig
		resource.ResourceName = group
		for _, key := range []string{"share.auto.offset.reset", "share.record.lock.duration.ms"} {
			entry := kmsg.NewIncrementalAlterConfigsRequestResourceConfig()
			entry.Name = key
			entry.Op = 1
			resource.Configs = append(resource.Configs, entry)
		}
		reset.Resources = []kmsg.IncrementalAlterConfigsRequestResource{resource}
		response, err = admin.Request(ctx, reset)
		if err != nil {
			t.Error(err)
		} else {
			for _, r := range response.(*kmsg.IncrementalAlterConfigsResponse).Resources {
				if err := kerr.ErrorForCode(r.ErrorCode); err != nil {
					t.Error(err)
				}
			}
		}
		deleteTopic := kmsg.NewPtrDeleteTopicsRequest()
		deleteTopic.TimeoutMillis = 10000
		entry := kmsg.NewDeleteTopicsRequestTopic()
		entry.Topic = kmsg.StringPtr(topic)
		deleteTopic.Topics = []kmsg.DeleteTopicsRequestTopic{entry}
		response, err = admin.Request(ctx, deleteTopic)
		if err != nil {
			t.Error(err)
		} else {
			for _, r := range response.(*kmsg.DeleteTopicsResponse).Topics {
				if err := kerr.ErrorForCode(r.ErrorCode); err != nil {
					t.Error(err)
				}
			}
		}
	})
	return config.KafkaShareConsumerConfig{Brokers: brokers, Topic: topic, GroupID: group, MaxPollRecords: 2, FetchMinBytes: 1, FetchMaxBytes: 1000000, FetchMaxWait: 50 * time.Millisecond, RequestTimeout: 5 * time.Second}, admin
}

// TestShareAppShutdownWaitsForHandlers exercises New, Start, and Shutdown with
// real runtime telemetry and Kafka, keeping reader cleanup behind handler exit.
func TestShareAppShutdownWaitsForHandlers(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	cfg, admin := integrationSubscription(t, ctx)
	t.Setenv("OTEL_EXPORT_ENABLED", "false")
	t.Setenv("KAFKA_BROKERS", brokers[0])
	t.Setenv("KAFKA_TOPIC", cfg.Topic)
	t.Setenv("KAFKA_SHARE_GROUP_ID", cfg.GroupID)
	t.Setenv("KAFKA_SHARE_MAX_POLL_RECORDS", "1")
	// Go test captures stderr through a pipe, which cannot be fsynced on some
	// platforms. Give the real logger a regular file so this tests lifecycle cleanup.
	logFile, err := os.CreateTemp(t.TempDir(), "worker-log-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := logFile.Close(); err != nil {
			t.Error(err)
		}
	})
	originalStderr := os.Stderr
	os.Stderr = logFile
	application, err := New(ctx)
	os.Stderr = originalStderr
	if err != nil {
		t.Fatal(err)
	}
	started, canceled, finish := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(finish) })
	usecase := &notificationtest.Usecase{HandleFunc: func(ctx context.Context, _ notification.Request) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-finish
		return ctx.Err()
	}}
	application.worker, err = kafkadelivery.NewNotificationShareWorker(shareReaderAdapter{application.reader}, usecase, nil, workerOptions())
	if err != nil {
		t.Fatal(err)
	}
	root, stop := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- application.Start(root) }()
	shutdownDone := false
	t.Cleanup(func() {
		if shutdownDone {
			return
		}
		stop()
		release.Do(func() { close(finish) })
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := application.Shutdown(cleanupCtx); err != nil {
			t.Error(err)
		}
	})
	produce(t, ctx, admin, cfg.Topic, `{"request_id":"shutdown","recipient":"user","content":"ready"}`)
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stop()
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// A canceled shutdown budget must report the wait instead of closing a reader
	// still owned by Run. A later call with a fresh budget can finish cleanup.
	expired, cancelExpired := context.WithCancel(ctx)
	cancelExpired()
	if err := application.Shutdown(expired); !errors.Is(err, context.Canceled) {
		t.Fatalf("early shutdown=%v", err)
	}
	release.Do(func() { close(finish) })
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cleanupCtx, cancelCleanup := context.WithTimeout(ctx, 10*time.Second)
	defer cancelCleanup()
	shutdownDone = true
	if err := application.Shutdown(cleanupCtx); err != nil {
		t.Fatal(err)
	}
	next := integrationReader(t, cfg)
	batch, err := next.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Records()[0].Offset != 0 || batch.Records()[0].DeliveryCount() < 2 {
		t.Fatal("shutdown did not release interrupted work")
	}
}
