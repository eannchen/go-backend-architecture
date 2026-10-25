package kafka

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestReaderSubscriptionPolicy protects manual commits, rebalance coordination,
// fetch buffering bounds, and the configured initial position for a new group.
func TestReaderSubscriptionPolicy(t *testing.T) {
	for _, start := range []config.KafkaInitialOffset{config.KafkaInitialEarliest, config.KafkaInitialLatest} {
		t.Run(string(start), func(t *testing.T) {
			cfg := validConsumerConfig()
			cfg.InitialOffset = start
			r, err := NewReader(cfg)
			if err != nil {
				t.Fatal(err)
			}
			cleanupReader(t, r)
			offset := kgo.NewOffset().AtStart()
			if start == config.KafkaInitialLatest {
				offset = kgo.NewOffset().AtEnd()
			}
			for _, check := range []struct{ option, want any }{
				{kgo.ConsumerGroup, cfg.GroupID}, {kgo.DisableAutoCommit, true}, {kgo.BlockRebalanceOnPoll, true},
				{kgo.ConsumeStartOffset, offset}, {kgo.ConsumeResetOffset, kgo.NoResetOffset()},
				{kgo.AllowAutoTopicCreation, false}, {kgo.FetchMinBytes, int32(cfg.FetchMinBytes)},
				{kgo.FetchMaxBytes, int32(cfg.FetchMaxBytes)}, {kgo.FetchMaxPartitionBytes, int32(cfg.FetchMaxBytes)},
				{kgo.FetchMaxWait, cfg.FetchMaxWait}, {kgo.RequestTimeoutOverhead, cfg.RequestTimeout},
				{kgo.MaxConcurrentFetches, cfg.MaxConcurrentFetches}, {kgo.RebalanceTimeout, cfg.RebalanceTimeout},
			} {
				if got := r.client.OptValue(check.option); !reflect.DeepEqual(got, check.want) {
					t.Fatalf("option %T=%v want=%v", check.option, got, check.want)
				}
			}
		})
	}
}

// TestReaderCancellationAndClosure preserves cancellation causes and rejects
// operations after close, including a canceled poll that blocked rebalances.
func TestReaderCancellationAndClosure(t *testing.T) {
	r, err := NewReader(validConsumerConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupReader(t, r)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.FetchRecord(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("fetch error=%v", err)
	}
	if err := r.CommitRecords(ctx, &kgo.Record{Topic: "events.v1", Offset: 10}); !errors.Is(err, context.Canceled) {
		t.Fatalf("commit error=%v", err)
	}
	if err := r.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.FetchRecord(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatalf("closed fetch=%v", err)
	}
	if err := r.CommitRecords(t.Context(), &kgo.Record{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed commit=%v", err)
	}
}

// TestReaderRejectsInvalidConfiguration prevents unsafe byte narrowing and SDK
// defaults from silently replacing an invalid subscription supplied directly.
func TestReaderRejectsInvalidConfiguration(t *testing.T) {
	for _, change := range []func(*config.KafkaConsumerConfig){
		func(c *config.KafkaConsumerConfig) { c.Brokers = nil }, func(c *config.KafkaConsumerConfig) { c.GroupID = "" },
		func(c *config.KafkaConsumerConfig) { c.FetchMaxBytes = 1 << 31 }, func(c *config.KafkaConsumerConfig) { c.MaxConcurrentFetches = 0 },
		func(c *config.KafkaConsumerConfig) { c.RequestTimeout = 0 }, func(c *config.KafkaConsumerConfig) { c.InitialOffset = "unknown" },
	} {
		cfg := validConsumerConfig()
		change(&cfg)
		r, err := NewReader(cfg)
		if r != nil {
			cleanupReader(t, r)
		}
		if r != nil || err == nil {
			t.Fatal("accepted invalid reader configuration")
		}
	}
}

func validConsumerConfig() config.KafkaConsumerConfig {
	return config.KafkaConsumerConfig{
		Brokers: []string{"127.0.0.1:1"}, Topic: "events.v1", GroupID: "worker-v1",
		FetchMinBytes: 1, FetchMaxBytes: 65536, FetchMaxWait: 500 * time.Millisecond,
		RequestTimeout: time.Second, RebalanceTimeout: time.Minute, MaxConcurrentFetches: 1, InitialOffset: config.KafkaInitialEarliest,
	}
}
func cleanupReader(t *testing.T, r *Reader) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := r.Close(ctx); err != nil {
			t.Error(err)
		}
	})
}
