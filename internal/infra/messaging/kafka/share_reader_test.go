package kafka

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestShareReaderPolicy protects bounded acquisition and share-specific settings
// rather than reusing the ordinary group's commit/rebalance mechanisms.
func TestShareReaderPolicy(t *testing.T) {
	cfg := validShareConfig()
	r, err := NewShareReader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	for _, check := range []struct{ option, want any }{
		{kgo.ShareGroup, cfg.GroupID}, {kgo.MaxConcurrentFetches, 0}, {kgo.ShareMaxRecords, int32(cfg.MaxPollRecords)},
		{kgo.ShareMaxRecordsStrict, true}, {kgo.BlockRebalanceOnPoll, false}, {kgo.DisableAutoCommit, false},
		{kgo.FetchMinBytes, int32(cfg.FetchMinBytes)}, {kgo.FetchMaxBytes, int32(cfg.FetchMaxBytes)},
		{kgo.FetchMaxWait, cfg.FetchMaxWait}, {kgo.RequestTimeoutOverhead, cfg.RequestTimeout}, {kgo.AllowAutoTopicCreation, false},
	} {
		if got := r.client.OptValue(check.option); !reflect.DeepEqual(got, check.want) {
			t.Fatalf("option %T=%v want=%v", check.option, got, check.want)
		}
	}
	bad := cfg
	bad.MaxPollRecords = 0
	if got, err := NewShareReader(bad); got != nil || err == nil {
		t.Fatal("accepted invalid share reader config")
	}
}

// TestShareBatchGuards protects the boundary before a second SDK poll can
// auto-accept unfinished records, and rejects foreign/stale/conflicting decisions.
func TestShareBatchGuards(t *testing.T) {
	r, err := NewShareReader(validShareConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	// Synthetic records exercise wrapper bookkeeping only. Real broker-backed
	// acknowledgment states and outcomes are covered by integration tests.
	a, b := &kgo.Record{}, &kgo.Record{}
	batch := newShareBatch(r, []*kgo.Record{a, b})
	r.active = batch
	if _, err := r.FetchBatch(t.Context()); !errors.Is(err, ErrShareBatchPending) {
		t.Fatalf("unfinished poll=%v", err)
	}
	if err := batch.Mark(a, "unknown"); err == nil {
		t.Fatal("accepted unknown decision")
	}
	if err := batch.Mark(&kgo.Record{}, ShareAccept); !errors.Is(err, ErrShareRecordNotInBatch) {
		t.Fatal(err)
	}
	copy := batch.Records()
	copy[0] = nil
	if batch.Records()[0] != a {
		t.Fatal("caller changed batch ownership through returned slice")
	}
	if err := batch.Mark(a, ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := batch.Mark(a, ShareAccept); err != nil {
		t.Fatal(err)
	}
	if err := batch.Mark(a, ShareReject); !errors.Is(err, ErrShareDecisionAlreadySet) {
		t.Fatal(err)
	}
	if _, err := r.FetchBatch(t.Context()); !errors.Is(err, ErrShareBatchPending) {
		t.Fatal("one undecided record must still block polling")
	}
	if err := batch.Mark(b, ShareRelease); err != nil {
		t.Fatal(err)
	}
	if batch.pending != 0 {
		t.Fatal("repeated marking corrupted pending count")
	}
	r.active = newShareBatch(r, nil)
	if err := batch.Mark(a, ShareAccept); !errors.Is(err, ErrShareBatchStale) {
		t.Fatal(err)
	}
	if err := r.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := batch.Mark(a, ShareAccept); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if _, err := r.FetchBatch(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

// TestShareBatchParallelHandlersReturnOutcomes shows parallel work with one
// owner marking the results, without concurrent access to reader state.
func TestShareBatchParallelHandlersReturnOutcomes(t *testing.T) {
	cfg := validShareConfig()
	cfg.MaxPollRecords = 4
	r, err := NewShareReader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	records := make([]*kgo.Record, cfg.MaxPollRecords)
	for i := range records {
		records[i] = &kgo.Record{Offset: int64(i)}
	}
	batch := newShareBatch(r, records)
	r.active = batch
	// Synthetic records test the ownership contract here; integration tests
	// verify actual Kafka acknowledgments with records returned by the broker.
	type outcome struct {
		record   *kgo.Record
		decision ShareDecision
	}
	results := make(chan outcome, len(records))
	var handlers sync.WaitGroup
	for _, record := range batch.Records() {
		handlers.Go(func() { results <- outcome{record: record, decision: ShareAccept} })
	}
	handlers.Wait()
	close(results)
	for result := range results {
		if err := batch.Mark(result.record, result.decision); err != nil {
			t.Fatal(err)
		}
	}
	if batch.pending != 0 {
		t.Fatalf("undecided=%d", batch.pending)
	}
}

// TestShareAcknowledgmentFailureReportedOnce ensures a queued rejection explains
// why fetching stopped, preserves its cause, and is not reported forever.
func TestShareAcknowledgmentFailureReportedOnce(t *testing.T) {
	r, err := NewShareReader(validShareConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	record := &kgo.Record{}
	batch := newShareBatch(r, []*kgo.Record{record})
	r.active = batch
	r.recordAckResults(r.client, kgo.ShareAckResults{{Topic: "jobs.v1", Partition: 0, Err: kerr.InvalidRecordState}})
	r.recordAckResults(r.client, kgo.ShareAckResults{{Topic: "jobs.v1", Partition: 0}})
	// Reporting one failure must not prevent decisions for the remaining work.
	if err := batch.Mark(record, ShareRelease); err != nil {
		t.Fatal(err)
	}
	_, err = r.FetchBatch(t.Context())
	if !errors.Is(err, ErrShareAcknowledgment) || !errors.Is(err, kerr.InvalidRecordState) {
		t.Fatalf("next fetch hid queued acknowledgment failure: %v", err)
	}
	if !strings.HasPrefix(err.Error(), "fetch share batch: flush previous acknowledgments before polling: ") {
		t.Fatalf("fetch error did not explain the failed step: %v", err)
	}
	if err := r.FlushAcknowledgments(t.Context()); err != nil {
		t.Fatalf("already reported error persisted: %v", err)
	}
	// A later failure must be reported independently, rather than erased by the
	// successful flush above or confused with the previous failed record.
	r.recordAckResults(r.client, kgo.ShareAckResults{{Topic: "jobs.v1", Partition: 0, Err: kerr.UnknownTopicID}})
	if err := r.FlushAcknowledgments(t.Context()); !errors.Is(err, kerr.UnknownTopicID) {
		t.Fatal(err)
	}
}

// TestShareReaderPreservesCancellation protects empty-topic waits and clean
// ownership when polling is canceled before a broker is contacted.
func TestShareReaderPreservesCancellation(t *testing.T) {
	r, err := NewShareReader(validShareConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.FetchBatch(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func validShareConfig() config.KafkaShareConsumerConfig {
	return config.KafkaShareConsumerConfig{
		Brokers: []string{"127.0.0.1:1"}, Topic: "jobs.v1", GroupID: "shared-jobs", MaxPollRecords: 2,
		FetchMinBytes: 1, FetchMaxBytes: 65536, FetchMaxWait: 100 * time.Millisecond, RequestTimeout: time.Second,
	}
}
func cleanupShareReader(t *testing.T, r *ShareReader) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.Close(ctx); err != nil {
			t.Error(err)
		}
	})
}
