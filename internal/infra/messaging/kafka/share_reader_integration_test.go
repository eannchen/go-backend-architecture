//go:build integration

package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// TestShareReadersShareOnePartition proves parallel acquisition is not limited
// to one consumer per partition, and each client's polled batch stays bounded.
func TestShareReadersShareOnePartition(t *testing.T) {
	topic := createIntegrationTopic(t, 1)
	group := createIntegrationShareGroup(t, topic+"-share", 30000)
	writer := newIntegrationWriter(t, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var published []*kgo.Record
	for i := 0; i < 6; i++ {
		published = append(published, &kgo.Record{Value: []byte(fmt.Sprintf("job-%d", i))})
	}
	if err := writer.WriteRecords(ctx, published...); err != nil {
		t.Fatal(err)
	}
	first := newIntegrationShareReader(t, topic, group, 2)
	batch1, err := first.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second := newIntegrationShareReader(t, topic, group, 2)
	batch2, err := second.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int64]bool)
	for _, batch := range []*ShareBatch{batch1, batch2} {
		if len(batch.Records()) < 1 || len(batch.Records()) > 2 {
			t.Fatal("poll exceeded configured bound")
		}
		for _, record := range batch.Records() {
			if record.Partition != 0 || seen[record.Offset] {
				t.Fatalf("record acquired twice before release: %+v", record)
			}
			seen[record.Offset] = true
			if record.DeliveryCount() != 1 {
				t.Fatalf("fresh record delivery count=%d", record.DeliveryCount())
			}
			if record.AcquisitionDeadline().IsZero() {
				t.Fatal("missing acquisition deadline")
			}
		}
	}
	// Neither consumer has finished its first batch yet, but both acquired records
	// from partition 0. Mark outcomes only after proving simultaneous ownership.
	for _, batch := range []*ShareBatch{batch1, batch2} {
		for _, record := range batch.Records() {
			if err := batch.Mark(record, ShareAccept); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := first.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
	if err := second.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestShareReleaseRejectAndIndependentGroup verifies per-record outcomes can
// complete out of order and rejecting work does not delete the retained message.
func TestShareReleaseRejectAndIndependentGroup(t *testing.T) {
	topic := createIntegrationTopic(t, 1)
	group := createIntegrationShareGroup(t, topic+"-share", 30000)
	writer := newIntegrationWriter(t, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("retry")}, &kgo.Record{Value: []byte("reject")}); err != nil {
		t.Fatal(err)
	}
	first := newIntegrationShareReader(t, topic, group, 2)
	batch, err := first.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records()) != 2 {
		t.Fatalf("expected two records in published batch, got %d", len(batch.Records()))
	}
	retry, reject := batch.Records()[0], batch.Records()[1]
	if err := batch.Mark(retry, ShareRelease); err != nil {
		t.Fatal(err)
	}
	if err := first.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := first.FetchBatch(ctx); !errors.Is(err, ErrShareBatchPending) {
		t.Fatal("unfinished record did not block next poll")
	}
	second := newIntegrationShareReader(t, topic, group, 1)
	again, err := second.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	record := again.Records()[0]
	if record.Offset != retry.Offset || record.DeliveryCount() < 2 {
		t.Fatal("released record was not redelivered")
	}
	if err := again.Mark(record, ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := second.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
	if err := batch.Mark(reject, ShareReject); err != nil {
		t.Fatal(err)
	}
	if err := first.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
	idleCtx, stop := context.WithTimeout(ctx, 750*time.Millisecond)
	_, err = first.FetchBatch(idleCtx)
	stop()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("accepted/rejected work delivered again: %v", err)
	}
	// A new share group can still read BOTH original records: reject did not
	// delete Kafka data or affect a different group's acknowledgments.
	otherGroup := createIntegrationShareGroup(t, topic+"-other", 30000)
	other := newIntegrationShareReader(t, topic, otherGroup, 2)
	independent, err := other.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(independent.Records()) != 2 {
		t.Fatal("independent group could not read retained records")
	}
	for _, record := range independent.Records() {
		if err := independent.Mark(record, ShareAccept); err != nil {
			t.Fatal(err)
		}
	}
	if err := other.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestShareClosePreservesMarkedAndReleasesUnfinished verifies shutdown confirms
// completed work and makes only unfinished work available for another attempt.
func TestShareClosePreservesMarkedAndReleasesUnfinished(t *testing.T) {
	topic := createIntegrationTopic(t, 1)
	group := createIntegrationShareGroup(t, topic+"-share", 30000)
	writer := newIntegrationWriter(t, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("done")}, &kgo.Record{Value: []byte("unfinished")}); err != nil {
		t.Fatal(err)
	}
	first := newIntegrationShareReader(t, topic, group, 2)
	batch, err := first.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records()) != 2 {
		t.Fatalf("expected two records, got %d", len(batch.Records()))
	}
	completed, unfinished := batch.Records()[0], batch.Records()[1]
	if err := batch.Mark(completed, ShareAccept); err != nil {
		t.Fatal(err)
	}
	// Do not explicitly flush: Close must confirm this acceptance itself.
	if _, err := first.FetchBatch(ctx); !errors.Is(err, ErrShareBatchPending) {
		t.Fatal(err)
	}
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	second := newIntegrationShareReader(t, topic, group, 1)
	replay, err := second.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	record := replay.Records()[0]
	if record.Offset != unfinished.Offset || record.DeliveryCount() < 2 {
		t.Fatal("shutdown did not preserve the accepted record and release the unfinished one")
	}
	if err := replay.Mark(record, ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := second.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(ctx); err != nil {
		t.Fatal(err)
	}
	third := newIntegrationShareReader(t, topic, group, 1)
	idleCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	if _, err := third.FetchBatch(idleCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("accepted work replayed after restart: %v", err)
	}
}

// TestShareLockExpiryAndRestartRecovery verifies redelivery, reporting a late
// acknowledgment failure, and the owner's explicit restart recovery.
func TestShareLockExpiryAndRestartRecovery(t *testing.T) {
	topic := createIntegrationTopic(t, 1)
	group := createIntegrationShareGroup(t, topic+"-share", 3000)
	writer := newIntegrationWriter(t, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("slow")}); err != nil {
		t.Fatal(err)
	}
	cfg := validShareConfig()
	cfg.Brokers = integrationBrokers
	cfg.Topic = topic
	cfg.GroupID = group
	cfg.MaxPollRecords = 1
	first, err := NewShareReader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, first)
	batch, err := first.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original := batch.Records()[0]
	second := newIntegrationShareReader(t, topic, group, 1)
	replay, err := second.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	record := replay.Records()[0]
	if record.Offset != original.Offset || record.DeliveryCount() < 2 {
		t.Fatal("expiry did not redeliver the locked record")
	}
	if err := batch.Mark(original, ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := first.FlushAcknowledgments(ctx); !errors.Is(err, ErrShareAcknowledgment) {
		t.Fatalf("late broker rejection was hidden: %v", err)
	}
	if err := replay.Mark(record, ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := second.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
	// With franz-go v1.22.2 + Kafka 4.2.1, this rejection can misroute subsequent
	// fetches to broker 0 (our broker is 1). Until that SDK behavior is fixed,
	// the owner explicitly recreates the reader instead of continuing to poll.
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	restarted := newIntegrationShareReader(t, topic, group, 1)
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("next")}); err != nil {
		t.Fatal(err)
	}
	next, err := restarted.FetchBatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.Records()[0].Offset != original.Offset+1 {
		t.Fatal("expected new work after restart")
	}
	if err := next.Mark(next.Records()[0], ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := restarted.FlushAcknowledgments(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestSharePollingCancellationThenClose verifies the owner can cancel an idle
// poll, wait for it to return, and only then shut down the reader.
func TestSharePollingCancellationThenClose(t *testing.T) {
	topic := createIntegrationTopic(t, 1)
	group := createIntegrationShareGroup(t, topic+"-share", 30000)
	reader := newIntegrationShareReader(t, topic, group, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	if _, err := reader.FetchBatch(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("idle poll=%v", err)
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := reader.Close(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

// TestShareCloseWaitsForFinalAcknowledgments protects shutdown's final release:
// report failure or cancellation, and always finish closing SDK resources.
func TestShareCloseWaitsForFinalAcknowledgments(t *testing.T) {
	for _, tc := range []struct {
		name       string
		cancelWait bool
	}{
		{name: "report final failure"}, {name: "cancel pending wait", cancelWait: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			topic := createIntegrationTopic(t, 1)
			group := createIntegrationShareGroup(t, topic+"-share", 30000)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			writer := newIntegrationWriter(t, topic)
			if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("unfinished")}); err != nil {
				t.Fatal(err)
			}
			callbackEntered := make(chan struct{})
			unblockCallback := make(chan struct{})
			callbackDone := make(chan struct{})
			sdkClosed := make(chan struct{})
			reader := &ShareReader{maxPollRecords: 1}
			client, err := kgo.NewClient(
				kgo.SeedBrokers(integrationBrokers...), kgo.ConsumeTopics(topic), kgo.ShareGroup(group),
				kgo.ShareMaxRecords(1), kgo.ShareMaxRecordsStrict(), kgo.MaxConcurrentFetches(0),
				kgo.WithHooks(shareClientClosedHook{done: sdkClosed}),
				kgo.ShareAckCallback(func(client *kgo.Client, results kgo.ShareAckResults) {
					close(callbackEntered)
					<-unblockCallback
					// Hold the real SDK callback and inject a rejection. This controls the
					// troublesome ordering without relying on a slow or broken broker.
					for i := range results {
						results[i].Err = kerr.InvalidRecordState
					}
					reader.recordAckResults(client, results)
					close(callbackDone)
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			reader.client = client
			closeCtx, cancelClose := context.WithCancel(ctx)
			defer cancelClose()
			var release sync.Once
			closeStarted := false
			closeDone := make(chan struct{})
			t.Cleanup(func() {
				release.Do(func() { close(unblockCallback) })
				cancelClose()
				cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				if closeStarted {
					// Join the owner even when an assertion ended the test early.
					select {
					case <-closeDone:
					case <-cleanupCtx.Done():
						t.Error("shutdown owner did not finish")
					}
				} else if err := reader.Close(cleanupCtx); err != nil {
					t.Error(err)
				}
			})
			if _, err := reader.FetchBatch(ctx); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			// Ownership moves to this goroutine for Close. Only contexts and callback
			// channels are controlled by the test; no reader calls overlap.
			closeStarted = true
			go func() { defer close(closeDone); result <- reader.Close(closeCtx) }()
			select {
			case <-callbackEntered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			// Hold the release callback until we either let its failure through or
			// cancel shutdown. Both paths must report an error and close the SDK.
			want := error(ErrShareAcknowledgment)
			if tc.cancelWait {
				// Cancel the callback wait after Kafka has processed the leave, rather
				// than canceling the leave request and waiting for session expiry.
				waitForIntegrationShareGroupEmpty(t, ctx, group)
				cancelClose()
				want = context.Canceled
			} else {
				release.Do(func() { close(unblockCallback) })
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("shutdown=%v want=%v", err, want)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case <-sdkClosed:
			default:
				t.Fatal("shutdown returned before closing SDK resources")
			}
			release.Do(func() { close(unblockCallback) })
			select {
			case <-callbackDone:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}

type shareClientClosedHook struct{ done chan struct{} }

func (h shareClientClosedHook) OnClientClosed(*kgo.Client) { close(h.done) }

func waitForIntegrationShareGroupEmpty(t *testing.T, ctx context.Context, group string) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		req := kmsg.NewPtrShareGroupDescribeRequest()
		req.GroupIDs = []string{group}
		response, err := integrationAdmin.Request(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		groups := response.(*kmsg.ShareGroupDescribeResponse).Groups
		if len(groups) != 1 {
			t.Fatalf("expected one described share group, got %d", len(groups))
		}
		if err := kerr.ErrorForCode(groups[0].ErrorCode); err != nil {
			t.Fatal(err)
		}
		if len(groups[0].Members) == 0 {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}

func createIntegrationShareGroup(t *testing.T, group string, lockMillis int) string {
	t.Helper()
	cleanupIntegrationGroup(t, group)
	cleanupIntegrationShareConfig(t, group)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	req := kmsg.NewPtrIncrementalAlterConfigsRequest()
	resource := kmsg.NewIncrementalAlterConfigsRequestResource()
	resource.ResourceType = kmsg.ConfigResourceTypeGroupConfig
	resource.ResourceName = group
	for key, value := range map[string]string{"share.auto.offset.reset": "earliest", "share.record.lock.duration.ms": fmt.Sprint(lockMillis)} {
		config := kmsg.NewIncrementalAlterConfigsRequestResourceConfig()
		config.Name = key
		config.Value = kmsg.StringPtr(value)
		resource.Configs = append(resource.Configs, config)
	}
	req.Resources = []kmsg.IncrementalAlterConfigsRequestResource{resource}
	response, err := integrationAdmin.Request(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range response.(*kmsg.IncrementalAlterConfigsResponse).Resources {
		if err := kerr.ErrorForCode(result.ErrorCode); err != nil {
			t.Fatal(err)
		}
	}
	return group
}

func newIntegrationShareReader(t *testing.T, topic, group string, maxRecords int) *ShareReader {
	t.Helper()
	cfg := validShareConfig()
	cfg.Brokers = integrationBrokers
	cfg.Topic = topic
	cfg.GroupID = group
	cfg.MaxPollRecords = maxRecords
	reader, err := NewShareReader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, reader)
	return reader
}

// TestShareGroupProvisioningScript runs the actual setup script twice against
// Kafka around actual record handling. This protects CLI compatibility and
// proves applying earliest again does not rewind this group's accepted progress.
func TestShareGroupProvisioningScript(t *testing.T) {
	topic := createIntegrationTopic(t, 1)
	group := topic + "-script"
	cleanupIntegrationGroup(t, group)
	cleanupIntegrationShareConfig(t, group)
	script, err := os.ReadFile("../../../../tools/kafka/init-share-group.sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	if err := integrationContainer.CopyToContainer(ctx, script, "/tmp/init-share-group.sh", 0o644); err != nil {
		t.Fatal(err)
	}
	writer := newIntegrationWriter(t, topic)
	for run := range 2 {
		code, output, err := integrationContainer.Exec(ctx, []string{"env", "KAFKA_BOOTSTRAP_SERVER=localhost:19092", "KAFKA_SHARE_GROUP_ID=" + group, "/bin/bash", "/tmp/init-share-group.sh"})
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(output)
		if err != nil {
			t.Fatal(err)
		}
		if code != 0 {
			t.Fatalf("group setup exit=%d: %s", code, body)
		}
		if !strings.Contains(string(body), "share.auto.offset.reset=earliest") || !strings.Contains(string(body), "share.record.lock.duration.ms=30000") {
			t.Fatalf("group settings missing: %s", body)
		}
		if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte(fmt.Sprintf("setup-run-%d", run))}); err != nil {
			t.Fatal(err)
		}
		reader := newIntegrationShareReader(t, topic, group, 1)
		batch, err := reader.FetchBatch(ctx)
		if err != nil {
			t.Fatal(err)
		}
		record := batch.Records()[0]
		if record.Offset != int64(run) {
			t.Fatalf("setup changed accepted progress: offset=%d want=%d", record.Offset, run)
		}
		if err := batch.Mark(record, ShareAccept); err != nil {
			t.Fatal(err)
		}
		if err := reader.FlushAcknowledgments(ctx); err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func cleanupIntegrationShareConfig(t *testing.T, group string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req := kmsg.NewPtrIncrementalAlterConfigsRequest()
		resource := kmsg.NewIncrementalAlterConfigsRequestResource()
		resource.ResourceType = kmsg.ConfigResourceTypeGroupConfig
		resource.ResourceName = group
		for _, key := range []string{"share.auto.offset.reset", "share.record.lock.duration.ms"} {
			entry := kmsg.NewIncrementalAlterConfigsRequestResourceConfig()
			entry.Name = key
			entry.Op = kmsg.IncrementalAlterConfigOpDelete
			resource.Configs = append(resource.Configs, entry)
		}
		req.Resources = []kmsg.IncrementalAlterConfigsRequestResource{resource}
		response, err := integrationAdmin.Request(ctx, req)
		if err != nil {
			t.Error(err)
			return
		}
		for _, result := range response.(*kmsg.IncrementalAlterConfigsResponse).Resources {
			if err := kerr.ErrorForCode(result.ErrorCode); err != nil {
				t.Error(err)
			}
		}
	})
}
