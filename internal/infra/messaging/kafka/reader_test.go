package kafka

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	kafkago "github.com/segmentio/kafka-go"
)

// TestNewReaderSubscriptionPolicy protects group-based work sharing, injected fetch bounds, and synchronous explicit commits.
func TestNewReaderSubscriptionPolicy(t *testing.T) {
	for _, start := range []config.KafkaStartOffset{config.KafkaStartEarliest, config.KafkaStartLatest} {
		t.Run(string(start), func(t *testing.T) {
			cfg := validConsumerConfig()
			cfg.StartOffset = start
			reader, err := NewReader(cfg)
			if err != nil {
				t.Fatal(err)
			}
			cleanupReader(t, reader)
			got := reader.reader.Config()
			if got.GroupID != cfg.GroupID || got.Topic != cfg.Topic || len(got.Brokers) != 1 || got.Brokers[0] != cfg.Brokers[0] {
				t.Fatalf("subscription differs: %+v", got)
			}
			if got.MinBytes != cfg.FetchMinBytes || got.MaxBytes != cfg.FetchMaxBytes || got.MaxWait != cfg.FetchMaxWait || got.ReadBatchTimeout != cfg.ReadBatchTimeout || got.QueueCapacity != cfg.QueueCapacity {
				t.Fatalf("fetch settings differ: %+v", got)
			}
			want := kafkago.FirstOffset
			if start == config.KafkaStartLatest {
				want = kafkago.LastOffset
			}
			if got.StartOffset != want || got.CommitInterval != 0 {
				t.Fatalf("offset/commit policy = %d/%v, want %d/0", got.StartOffset, got.CommitInterval, want)
			}
		})
	}
}

// TestReaderPreservesCancellation protects blocked fetch/commit callers without requiring a broker.
func TestReaderPreservesCancellation(t *testing.T) {
	reader, err := NewReader(validConsumerConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupReader(t, reader)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := reader.FetchMessage(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("fetch error = %v, want cancellation", err)
	}
	if err := reader.CommitMessages(ctx, kafkago.Message{Topic: "events.v1", Partition: 0, Offset: 10}); !errors.Is(err, context.Canceled) {
		t.Fatalf("commit error = %v, want cancellation", err)
	}
}

// TestReaderCloseStopsOperations protects explicit lifecycle ownership and prevents commits after shutdown.
func TestReaderCloseStopsOperations(t *testing.T) {
	reader, err := NewReader(validConsumerConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupReader(t, reader)
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := reader.FetchMessage(ctx); !errors.Is(err, io.EOF) {
		t.Fatalf("closed fetch error = %v, want EOF", err)
	}
	// Repeat to catch the SDK's select race between a closed subscription and a
	// still-writable buffered commit queue. The wrapper must always reject it.
	for range 20 {
		if err := reader.CommitMessages(ctx, kafkago.Message{Topic: "events.v1", Partition: 0, Offset: 10}); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("closed commit error = %v, want closed pipe", err)
		}
	}
}

// TestNewReaderRejectsInvalidConfiguration protects direct callers from constructor panics and SDK fallback settings.
func TestNewReaderRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		change func(*config.KafkaConsumerConfig)
	}{
		{"no brokers", func(c *config.KafkaConsumerConfig) { c.Brokers = nil }},
		{"blank broker", func(c *config.KafkaConsumerConfig) { c.Brokers = []string{" "} }},
		{"blank topic", func(c *config.KafkaConsumerConfig) { c.Topic = " " }},
		{"blank group", func(c *config.KafkaConsumerConfig) { c.GroupID = " " }},
		{"zero min bytes", func(c *config.KafkaConsumerConfig) { c.FetchMinBytes = 0 }},
		{"negative min bytes", func(c *config.KafkaConsumerConfig) { c.FetchMinBytes = -1 }},
		{"zero max bytes", func(c *config.KafkaConsumerConfig) { c.FetchMaxBytes = 0 }},
		{"negative max bytes", func(c *config.KafkaConsumerConfig) { c.FetchMaxBytes = -1 }},
		{"inverted fetch bounds", func(c *config.KafkaConsumerConfig) { c.FetchMinBytes = c.FetchMaxBytes + 1 }},
		{"zero wait", func(c *config.KafkaConsumerConfig) { c.FetchMaxWait = 0 }},
		{"negative wait", func(c *config.KafkaConsumerConfig) { c.FetchMaxWait = -time.Second }},
		{"zero batch timeout", func(c *config.KafkaConsumerConfig) { c.ReadBatchTimeout = 0 }},
		{"negative batch timeout", func(c *config.KafkaConsumerConfig) { c.ReadBatchTimeout = -time.Second }},
		{"zero queue", func(c *config.KafkaConsumerConfig) { c.QueueCapacity = 0 }},
		{"negative queue", func(c *config.KafkaConsumerConfig) { c.QueueCapacity = -1 }},
		{"missing offset", func(c *config.KafkaConsumerConfig) { c.StartOffset = "" }},
		{"unknown offset", func(c *config.KafkaConsumerConfig) { c.StartOffset = "unknown" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConsumerConfig()
			tt.change(&cfg)
			reader, err := NewReader(cfg)
			if reader != nil {
				cleanupReader(t, reader)
			}
			if err == nil || reader != nil {
				t.Fatalf("NewReader() = (%v, %v), want rejected config", reader, err)
			}
		})
	}
}

func validConsumerConfig() config.KafkaConsumerConfig {
	// The closed port keeps unit tests independent of Kafka; Close owns the SDK's group goroutines.
	return config.KafkaConsumerConfig{
		Brokers: []string{"127.0.0.1:1"}, Topic: "events.v1", GroupID: "worker-v1",
		FetchMinBytes: 1, FetchMaxBytes: 65536, FetchMaxWait: 500 * time.Millisecond,
		ReadBatchTimeout: time.Second, QueueCapacity: 10, StartOffset: config.KafkaStartEarliest,
	}
}

func cleanupReader(t *testing.T, reader *Reader) {
	t.Helper()
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}
