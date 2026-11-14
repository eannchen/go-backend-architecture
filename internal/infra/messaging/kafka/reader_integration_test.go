//go:build integration

package kafka

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka/kafkatest"

	"github.com/testcontainers/testcontainers-go"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var integrationBackend *kafkatest.Broker
var integrationBrokers []string
var integrationAdmin *kgo.Client
var integrationContainer testcontainers.Container

// TestMain owns this package's single broker. Each test owns its backend data.
func TestMain(m *testing.M) {
	os.Exit(kafkatest.RunSuite(m, func(b *kafkatest.Broker) {
		integrationBackend = b
		integrationBrokers = b.Brokers
		integrationAdmin = b.Admin
		integrationContainer = b.Container
	}))
}

func createIntegrationTopic(t *testing.T, partitions int32) string {
	t.Helper()
	return integrationBackend.CreateTopic(t, partitions)
}
func cleanupIntegrationGroup(t *testing.T, group string) {
	t.Helper()
	integrationBackend.CleanupGroup(t, group)
}

// TestKafkaRestartReplaysOnlyUncommitted verifies closing after failed handling
// cannot auto-commit a fetched record, and a successful commit survives restart.
func TestKafkaRestartReplaysOnlyUncommitted(t *testing.T) {
	topic := createIntegrationTopic(t, 1)
	group := topic + "-group"
	cleanupIntegrationGroup(t, group)
	writer := newIntegrationWriter(t, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("first")}); err != nil {
		t.Fatal(err)
	}
	first := newIntegrationReader(t, topic, group)
	record, err := first.FetchRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately leave the rebalance block held, as if handling failed. Close
	// must release it and stop without committing, rather than hanging on leave.
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	second := newIntegrationReader(t, topic, group)
	replay, err := second.FetchRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Offset != record.Offset || string(replay.Value) != "first" {
		t.Fatal("uncommitted record not replayed")
	}
	if err := second.CommitRecords(ctx, replay); err != nil {
		t.Fatal(err)
	}
	second.AllowRebalance()
	if err := second.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writer = newIntegrationWriter(t, topic)
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("second")}); err != nil {
		t.Fatal(err)
	}
	third := newIntegrationReader(t, topic, group)
	next, err := third.FetchRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.Offset != record.Offset+1 || string(next.Value) != "second" {
		t.Fatal("committed progress not retained")
	}
	if err := third.CommitRecords(ctx, next); err != nil {
		t.Fatal(err)
	}
	third.AllowRebalance()
}

// TestKafkaRebalanceWaitsForProcessing verifies a second consumer can join while
// the first safely commits its polled record, then receive work after release.
func TestKafkaRebalanceWaitsForProcessing(t *testing.T) {
	topic := createIntegrationTopic(t, 2)
	group := topic + "-group"
	cleanupIntegrationGroup(t, group)
	writer := newIntegrationWriter(t, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("initial-0")}, &kgo.Record{Value: []byte("initial-1")}); err != nil {
		t.Fatal(err)
	}
	first := newIntegrationReader(t, topic, group)
	record, err := first.FetchRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second := newIntegrationReader(t, topic, group)
	type fetched struct {
		record *kgo.Record
		err    error
	}
	result := make(chan fetched, 1)
	fetchCtx, stopFetch := context.WithCancel(ctx)
	defer stopFetch()
	go func() { r, err := second.FetchRecord(fetchCtx); result <- fetched{r, err} }()
	// Join ownership: always cancel and wait for the polling goroutine before
	// reader cleanup, including a failed assertion before the rebalance releases.
	joined := false
	defer func() {
		stopFetch()
		if !joined {
			select {
			case <-result:
			case <-time.After(5 * time.Second):
				t.Error("polling goroutine did not stop")
			}
		}
	}()
	waitForPreparingRebalance(t, ctx, group)
	if err := first.CommitRecords(ctx, record); err != nil {
		t.Fatal(err)
	}
	first.AllowRebalance()
	// Give both partitions new records so whichever partition moves has work.
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("after-0")}, &kgo.Record{Value: []byte("after-1")}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-result:
		joined = true
		if got.err != nil {
			t.Fatal(got.err)
		}
		if err := second.CommitRecords(ctx, got.record); err != nil {
			t.Fatal(err)
		}
		second.AllowRebalance()
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func newIntegrationReader(t *testing.T, topic, group string) *Reader {
	t.Helper()
	cfg := validConsumerConfig()
	cfg.Brokers = integrationBrokers
	cfg.Topics = []string{topic}
	cfg.GroupID = group
	reader, err := NewReader(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanupReader(t, reader)
	return reader
}
func waitForPreparingRebalance(t *testing.T, ctx context.Context, group string) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		req := kmsg.NewPtrDescribeGroupsRequest()
		req.Groups = []string{group}
		response, err := integrationAdmin.Request(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range response.(*kmsg.DescribeGroupsResponse).Groups {
			if entry.State == "PreparingRebalance" {
				return
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal(fmt.Errorf("waiting for actual rebalance state: %w", ctx.Err()))
		}
	}
}
