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

// TestWriterPolicies protects explicit topic ownership, bounded delivery/retries,
// and the relationship between acknowledgments and producer idempotency.
func TestWriterPolicies(t *testing.T) {
	for _, tc := range []struct {
		policy             config.KafkaAcknowledgments
		acks               kgo.Acks
		disableIdempotency bool
	}{
		{config.KafkaAcksAll, kgo.AllISRAcks(), false},
		{config.KafkaAcksLeader, kgo.LeaderAck(), true},
		{config.KafkaAcksNone, kgo.NoAck(), true},
	} {
		t.Run(string(tc.policy), func(t *testing.T) {
			cfg := validPublisherConfig()
			cfg.RequiredAcks = tc.policy
			w, err := NewWriter(cfg)
			if err != nil {
				t.Fatal(err)
			}
			cleanupWriter(t, w)
			for _, check := range []struct{ option, want any }{
				{kgo.DefaultProduceTopic, cfg.Topic}, {kgo.RequiredAcks, tc.acks},
				{kgo.DisableIdempotentWrite, tc.disableIdempotency},
				{kgo.AllowIdempotentProduceCancellation, true}, {kgo.AllowAutoTopicCreation, false},
				{kgo.RequestTimeoutOverhead, cfg.RequestTimeout}, {kgo.ProduceRequestTimeout, cfg.ProduceTimeout},
				{kgo.RecordDeliveryTimeout, cfg.DeliveryTimeout}, {kgo.ProducerLinger, cfg.BatchTimeout},
				{kgo.ProducerBatchMaxBytes, int32(cfg.BatchBytes)}, {kgo.MaxBufferedRecords, int64(cfg.MaxBufferedRecords)},
				{kgo.RecordRetries, int64(cfg.MaxAttempts - 1)}, {kgo.UnknownTopicRetries, int64(cfg.MaxAttempts - 1)},
			} {
				if got := w.client.OptValue(check.option); !reflect.DeepEqual(got, check.want) {
					t.Fatalf("option %T = %v, want %v", check.option, got, check.want)
				}
			}
		})
	}
}

// TestWriterPartitioning preserves the previous key mapping across the SDK
// migration, including signed hashes, empty keys, and keyless round-robin routing.
func TestWriterPartitioning(t *testing.T) {
	cfg := validPublisherConfig()
	cfg.PartitionStrategy = config.PartitionByKey
	w, err := NewWriter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, w)
	p := w.client.OptValue(kgo.RecordPartitioner).(kgo.Partitioner).ForTopic(cfg.Topic)
	// Fixed results were computed from kafka-go v0.4.51 Hash before migration.
	for _, tc := range []struct {
		key              string
		partitions, want int
	}{
		{"recipient-123", 3, 0}, {"recipient-123", 5, 2}, {"", 3, 0}, {"user-1", 5, 1},
	} {
		r := &kgo.Record{Key: []byte(tc.key)}
		if got := p.Partition(r, tc.partitions); got != tc.want {
			t.Fatalf("key=%q partitions=%d got=%d want=%d", tc.key, tc.partitions, got, tc.want)
		}
		if !p.RequiresConsistency(r) {
			t.Fatal("keyed routing must consider all partitions")
		}
	}
	for i := 0; i < 6; i++ {
		if got := p.Partition(&kgo.Record{}, 3); got != i%3 {
			t.Fatalf("keyless partition=%d want=%d", got, i%3)
		}
	}
	cfg.PartitionStrategy = config.PartitionRoundRobin
	rr, err := NewWriter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, rr)
	rp := rr.client.OptValue(kgo.RecordPartitioner).(kgo.Partitioner).ForTopic(cfg.Topic)
	for i := 0; i < 6; i++ {
		if got := rp.Partition(&kgo.Record{Key: []byte("same-key")}, 3); got != i%3 {
			t.Fatalf("round-robin partition=%d want=%d", got, i%3)
		}
	}
}

// TestWriterCompression keeps the config-owned codec mapping explicit rather
// than inheriting franz-go's default compression choice.
func TestWriterCompression(t *testing.T) {
	for _, tc := range []struct {
		codec config.KafkaCompression
		want  kgo.CompressionCodec
	}{
		{config.CompressionNone, kgo.NoCompression()}, {config.CompressionGzip, kgo.GzipCompression()},
		{config.CompressionSnappy, kgo.SnappyCompression()}, {config.CompressionLz4, kgo.Lz4Compression()},
		{config.CompressionZstd, kgo.ZstdCompression()},
	} {
		t.Run(string(tc.codec), func(t *testing.T) {
			cfg := validPublisherConfig()
			cfg.Compression = tc.codec
			w, err := NewWriter(cfg)
			if err != nil {
				t.Fatal(err)
			}
			cleanupWriter(t, w)
			if got := w.client.OptValue(kgo.ProducerBatchCompression); !reflect.DeepEqual(got, []kgo.CompressionCodec{tc.want}) {
				t.Fatalf("compression=%v want=%v", got, tc.want)
			}
		})
	}
}

// TestWriterCancellationAndClosure prevents publishing with an ended context or
// closed client and verifies independent connection ownership without a broker.
func TestWriterCancellationAndClosure(t *testing.T) {
	w, err := NewWriter(validPublisherConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, w)
	other, err := NewWriter(validPublisherConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, other)
	if w.client == other.client {
		t.Fatal("writers share a client")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := w.WriteRecords(ctx, &kgo.Record{Value: []byte("canceled")}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRecords(t.Context(), &kgo.Record{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed error=%v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestWriterCancellationWhileWaiting protects the delivery boundary after a
// record was queued, rather than only checking contexts canceled before a call.
func TestWriterCancellationWhileWaiting(t *testing.T) {
	w, err := NewWriter(validPublisherConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, w)
	// The closed loopback port cannot acknowledge publication. The caller's
	// deadline must still end the queued operation and preserve its cause.
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	err = w.WriteRecords(ctx, &kgo.Record{Value: []byte("waiting")})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued publication error = %v, want caller deadline", err)
	}
}

// TestWriterRejectsInvalidInput ensures invalid config and conflicting record
// topics fail before any broker I/O or partial submission of a multi-record call.
func TestWriterRejectsInvalidInput(t *testing.T) {
	for _, change := range []func(*config.KafkaPublisherConfig){
		func(c *config.KafkaPublisherConfig) { c.Brokers = nil }, func(c *config.KafkaPublisherConfig) { c.RequiredAcks = "unknown" },
		func(c *config.KafkaPublisherConfig) { c.BatchBytes = 1 << 31 }, func(c *config.KafkaPublisherConfig) { c.MaxAttempts = 0 },
		func(c *config.KafkaPublisherConfig) { c.Compression = "unknown" }, func(c *config.KafkaPublisherConfig) { c.DeliveryTimeout = 0 },
	} {
		cfg := validPublisherConfig()
		change(&cfg)
		w, err := NewWriter(cfg)
		if w != nil {
			cleanupWriter(t, w)
		}
		if w != nil || err == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
	w, err := NewWriter(validPublisherConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, w)
	for _, records := range [][]*kgo.Record{{nil}, {{Value: []byte("valid")}, {Topic: "other-topic"}}} {
		if err := w.WriteRecords(t.Context(), records...); err == nil {
			t.Fatal("accepted invalid records")
		}
	}
}

// TestRetryBackoffBounds protects bounded jitter even for large retry counts
// and durations that would overflow if multiplied before checking the cap.
func TestRetryBackoffBounds(t *testing.T) {
	for _, limits := range [][2]time.Duration{{time.Millisecond, time.Second}, {time.Second, time.Second}, {1 << 61, 1 << 62}} {
		backoff := retryBackoff(limits[0], limits[1])
		for _, attempt := range []int{0, 1, 2, 1000000} {
			for range 20 {
				got := backoff(attempt)
				if got < limits[0] || got > limits[1] {
					t.Fatalf("backoff %v outside %v", got, limits)
				}
			}
		}
	}
}

func validPublisherConfig() config.KafkaPublisherConfig {
	return config.KafkaPublisherConfig{
		Brokers: []string{"127.0.0.1:1"}, Topic: "events.v1", PartitionStrategy: config.PartitionRoundRobin, RequiredAcks: config.KafkaAcksAll,
		RequestTimeout: time.Second, ProduceTimeout: time.Second, DeliveryTimeout: 5 * time.Second,
		MaxBufferedRecords: 25, BatchBytes: 65536, BatchTimeout: time.Millisecond, MaxAttempts: 3,
		WriteBackoffMin: 10 * time.Millisecond, WriteBackoffMax: 100 * time.Millisecond, Compression: config.CompressionNone,
	}
}
func cleanupWriter(t *testing.T, w *Writer) {
	t.Helper()
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
}
