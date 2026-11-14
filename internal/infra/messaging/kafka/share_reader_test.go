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
	r, err := NewShareReader(cfg, nil)
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
	if got, err := NewShareReader(bad, nil); got != nil || err == nil {
		t.Fatal("accepted invalid share reader config")
	}
}

// TestShareReaderParallelHandlersReturnOutcomes shows parallel work with one
// owner marking the results, without concurrent access to reader state.
func TestShareReaderParallelHandlersReturnOutcomes(t *testing.T) {
	cfg := validShareConfig()
	cfg.MaxPollRecords = 4
	r, err := NewShareReader(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	records := make([]*kgo.Record, cfg.MaxPollRecords)
	for i := range records {
		records[i] = &kgo.Record{Offset: int64(i)}
	}
	batch := newShareBatch(records)
	r.active = batch
	// Synthetic records test the ownership contract here; integration tests
	// verify actual Kafka acknowledgments with records returned by the broker.
	results := make(chan *kgo.Record, len(records))
	var handlers sync.WaitGroup
	for _, record := range records {
		handlers.Go(func() { results <- record })
	}
	handlers.Wait()
	close(results)
	for record := range results {
		if err := r.Accept(record); err != nil {
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
	r, err := NewShareReader(validShareConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	record := &kgo.Record{}
	batch := newShareBatch([]*kgo.Record{record})
	r.active = batch
	r.recordAckResults(r.client, kgo.ShareAckResults{{Topic: "jobs.v1", Partition: 0, Err: kerr.InvalidRecordState}})
	r.recordAckResults(r.client, kgo.ShareAckResults{{Topic: "jobs.v1", Partition: 0}})
	// Reporting one failure must not prevent decisions for the remaining work.
	if err := r.Release(record); err != nil {
		t.Fatal(err)
	}
	_, err = r.FetchRecords(t.Context())
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
	r, err := NewShareReader(validShareConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.FetchRecords(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func validShareConfig() config.KafkaShareConsumerConfig {
	return config.KafkaShareConsumerConfig{
		Brokers: []string{"127.0.0.1:1"}, Topics: []string{"jobs.v1"}, GroupID: "shared-jobs", MaxPollRecords: 2,
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

// TestShareReaderRecordDecisions protects direct reader decisions from bypassing
// batch membership, duplicate-decision checks, or the unfinished-work poll guard.
func TestShareReaderRecordDecisions(t *testing.T) {
	r, err := NewShareReader(validShareConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanupShareReader(t, r)
	record := &kgo.Record{}
	for _, mark := range []func(*kgo.Record) error{r.Accept, r.Release, r.Reject} {
		if err := mark(record); !errors.Is(err, ErrShareRecordNotInBatch) {
			t.Fatalf("before fetch: %v", err)
		}
	}
	if err := r.ReleaseRemaining(); err != nil {
		t.Fatal(err)
	}
	records := []*kgo.Record{{Offset: 0}, {Offset: 1}, {Offset: 2}, {Offset: 3}}
	r.active = newShareBatch(records)
	if _, err := r.FetchRecords(t.Context()); !errors.Is(err, ErrShareBatchPending) {
		t.Fatal(err)
	}
	for i, mark := range []func(*kgo.Record) error{r.Accept, r.Release, r.Reject} {
		if err := mark(records[i]); err != nil {
			t.Fatal(err)
		}
		if err := mark(records[i]); err != nil {
			t.Fatalf("repeating decision: %v", err)
		}
	}
	if err := r.Reject(records[0]); !errors.Is(err, ErrShareDecisionAlreadySet) {
		t.Fatal(err)
	}
	if err := r.Accept(record); !errors.Is(err, ErrShareRecordNotInBatch) {
		t.Fatal(err)
	}
	if r.active.pending != 1 {
		t.Fatalf("pending=%d", r.active.pending)
	}
	for range 2 {
		if err := r.ReleaseRemaining(); err != nil {
			t.Fatal(err)
		}
	}
	if r.active.pending != 0 {
		t.Fatalf("pending after release=%d", r.active.pending)
	}
	for i, want := range []kgo.AckStatus{kgo.AckAccept, kgo.AckRelease, kgo.AckReject, kgo.AckRelease} {
		if got := r.active.decisions[records[i]]; got != want {
			t.Fatalf("record %d decision=%v want=%v", i, got, want)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.FetchRecords(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// Moving to another batch must not allow late outcomes for earlier records.
	r.active = newShareBatch([]*kgo.Record{record})
	if err := r.Accept(records[0]); !errors.Is(err, ErrShareRecordNotInBatch) {
		t.Fatal(err)
	}
	if err := r.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, mark := range []func(*kgo.Record) error{r.Accept, r.Release, r.Reject} {
		if err := mark(record); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("after close: %v", err)
		}
	}
	if err := r.ReleaseRemaining(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if _, err := r.FetchRecords(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
