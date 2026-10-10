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

// TestNewWriterPartitionStrategy protects caller-selected routing rather than imposing the example's key policy.
func TestNewWriterPartitionStrategy(t *testing.T) {
	for _, strategy := range []config.PartitionStrategy{config.PartitionByKey, config.PartitionRoundRobin} {
		t.Run(string(strategy), func(t *testing.T) {
			cfg := validPublisherConfig()
			cfg.PartitionStrategy = strategy
			writer, err := NewWriter(cfg)
			if err != nil {
				t.Fatalf("NewWriter() error = %v", err)
			}
			cleanupWriter(t, writer)
			// Check our SDK mapping; the SDK owns the hashing and rotation algorithms.
			switch strategy {
			case config.PartitionByKey:
				if _, ok := writer.writer.Balancer.(*kafkago.Hash); !ok {
					t.Fatalf("balancer = %T, want *kafkago.Hash", writer.writer.Balancer)
				}
			case config.PartitionRoundRobin:
				if _, ok := writer.writer.Balancer.(*kafkago.RoundRobin); !ok {
					t.Fatalf("balancer = %T, want *kafkago.RoundRobin", writer.writer.Balancer)
				}
			}
		})
	}
}

// TestNewWriterPublicationPolicy protects acknowledged publication, explicit provisioning, and injected deadlines.
func TestNewWriterPublicationPolicy(t *testing.T) {
	cfg := validPublisherConfig()
	cfg.Brokers = []string{"localhost:9092", "localhost:19092"}
	writer, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	cleanupWriter(t, writer)
	if writer.writer.Topic != cfg.Topic {
		t.Fatalf("topic = %q, want %q", writer.writer.Topic, cfg.Topic)
	}
	if writer.writer.Addr.String() != kafkago.TCP("localhost:9092", "localhost:19092").String() {
		t.Fatalf("broker addresses = %v", writer.writer.Addr)
	}
	if writer.writer.Async || writer.writer.RequiredAcks != kafkago.RequireAll || writer.writer.AllowAutoTopicCreation {
		t.Fatalf("publication policy = (async %v, acks %v, auto-topic %v), want false/all/false", writer.writer.Async, writer.writer.RequiredAcks, writer.writer.AllowAutoTopicCreation)
	}
	if writer.writer.ReadTimeout != cfg.ReadTimeout || writer.writer.WriteTimeout != cfg.WriteTimeout {
		t.Fatalf("network timeouts = %v/%v, want %v/%v", writer.writer.ReadTimeout, writer.writer.WriteTimeout, cfg.ReadTimeout, cfg.WriteTimeout)
	}
	if writer.writer.BatchSize != cfg.BatchSize || writer.writer.BatchBytes != cfg.BatchBytes || writer.writer.BatchTimeout != cfg.BatchTimeout || writer.writer.MaxAttempts != cfg.MaxAttempts || writer.writer.WriteBackoffMin != cfg.WriteBackoffMin || writer.writer.WriteBackoffMax != cfg.WriteBackoffMax {
		t.Fatalf("writer did not preserve batching and retry settings: %+v", writer.writer)
	}

}

// TestNewWriterPreservesCallerCancellation protects shutdown by preserving a cancelled publication's context error.
func TestNewWriterPreservesCallerCancellation(t *testing.T) {
	// The closed port keeps this check independent of a running broker.
	cfg := validPublisherConfig()
	cfg.Brokers = []string{"127.0.0.1:1"}
	writer, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	cleanupWriter(t, writer)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = writer.WriteMessages(ctx, kafkago.Message{Key: []byte("recipient-123"), Value: []byte("notification")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteMessages() error = %v, want context cancellation", err)
	}
}

// TestNewWriterRejectsInvalidConfiguration protects direct callers that bypass environment loading from SDK fallback policies.
func TestNewWriterRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		change func(*[]string, *config.KafkaPublisherConfig)
	}{
		{"no brokers", func(b *[]string, _ *config.KafkaPublisherConfig) { *b = nil }},
		{"empty broker", func(b *[]string, _ *config.KafkaPublisherConfig) { *b = []string{""} }},
		{"blank broker", func(b *[]string, _ *config.KafkaPublisherConfig) { *b = []string{"   "} }},
		{"empty broker among valid brokers", func(b *[]string, _ *config.KafkaPublisherConfig) { *b = []string{"localhost:9092", ""} }},
		{"empty topic", func(_ *[]string, c *config.KafkaPublisherConfig) { c.Topic = "" }},
		{"blank topic", func(_ *[]string, c *config.KafkaPublisherConfig) { c.Topic = "   " }},
		{"missing strategy", func(_ *[]string, c *config.KafkaPublisherConfig) { c.PartitionStrategy = "" }},
		{"unknown strategy", func(_ *[]string, c *config.KafkaPublisherConfig) { c.PartitionStrategy = "unknown" }},
		{"zero read timeout", func(_ *[]string, c *config.KafkaPublisherConfig) { c.ReadTimeout = 0 }},
		{"negative read timeout", func(_ *[]string, c *config.KafkaPublisherConfig) { c.ReadTimeout = -time.Second }},
		{"zero write timeout", func(_ *[]string, c *config.KafkaPublisherConfig) { c.WriteTimeout = 0 }},
		{"negative write timeout", func(_ *[]string, c *config.KafkaPublisherConfig) { c.WriteTimeout = -time.Second }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokers := []string{"localhost:9092"}
			cfg := validPublisherConfig()
			tt.change(&brokers, &cfg)
			cfg.Brokers = brokers
			writer, err := NewWriter(cfg)
			if writer != nil {
				cleanupWriter(t, writer)
			}
			if err == nil || writer != nil {
				t.Fatalf("NewWriter() = (%v, %v), want nil writer and configuration error", writer, err)
			}
		})
	}
}

func validPublisherConfig() config.KafkaPublisherConfig {
	return config.KafkaPublisherConfig{
		Brokers:           []string{"localhost:9092"},
		Topic:             "notifications.requested.v1",
		PartitionStrategy: config.PartitionRoundRobin,
		RequiredAcks:      config.KafkaAcksAll,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		BatchSize:         25,
		BatchBytes:        65536,
		BatchTimeout:      50 * time.Millisecond,
		MaxAttempts:       3,
		WriteBackoffMin:   10 * time.Millisecond,
		WriteBackoffMax:   100 * time.Millisecond,
		Compression:       config.CompressionNone,
	}
}

func cleanupWriter(t *testing.T, writer *Writer) {
	t.Helper()
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

// TestNewWriterCompression protects the config-owned codec mapping without requiring vendor types in config.
func TestNewWriterCompression(t *testing.T) {
	tests := []struct {
		codec config.KafkaCompression
		want  kafkago.Compression
	}{
		{config.CompressionNone, 0}, {config.CompressionGzip, kafkago.Gzip},
		{config.CompressionSnappy, kafkago.Snappy}, {config.CompressionLz4, kafkago.Lz4},
		{config.CompressionZstd, kafkago.Zstd},
	}
	for _, tt := range tests {
		t.Run(string(tt.codec), func(t *testing.T) {
			cfg := validPublisherConfig()
			cfg.Compression = tt.codec
			writer, err := NewWriter(cfg)
			if err != nil {
				t.Fatal(err)
			}
			cleanupWriter(t, writer)
			if writer.writer.Compression != tt.want {
				t.Fatalf("compression = %v, want %v", writer.writer.Compression, tt.want)
			}
		})
	}
}

// TestWriterOwnsTransportAndRejectsWritesAfterClose protects independent pool ownership and prevents publication during shutdown.
func TestWriterOwnsTransportAndRejectsWritesAfterClose(t *testing.T) {
	first, err := NewWriter(validPublisherConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, first)
	second, err := NewWriter(validPublisherConfig())
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, second)
	if first.transport == second.transport || first.writer.Transport != first.transport || second.writer.Transport != second.transport {
		t.Fatal("writers must each own their assigned transport")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	err = first.WriteMessages(t.Context(), kafkago.Message{Value: []byte("closed")})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed publication error = %v, want io.ErrClosedPipe", err)
	}
	// No broker was contacted; this protects ownership and closure, not real pool cleanup or flushing.
}

// TestNewWriterRejectsInvalidTuning protects direct callers from invalid batching, retries, and unsupported codecs.
func TestNewWriterRejectsInvalidTuning(t *testing.T) {
	tests := []struct {
		name   string
		change func(*config.KafkaPublisherConfig)
	}{
		{"zero batch size", func(c *config.KafkaPublisherConfig) { c.BatchSize = 0 }},
		{"negative batch size", func(c *config.KafkaPublisherConfig) { c.BatchSize = -1 }},
		{"zero batch bytes", func(c *config.KafkaPublisherConfig) { c.BatchBytes = 0 }},
		{"negative batch bytes", func(c *config.KafkaPublisherConfig) { c.BatchBytes = -1 }},
		{"zero batch timeout", func(c *config.KafkaPublisherConfig) { c.BatchTimeout = 0 }},
		{"zero attempts", func(c *config.KafkaPublisherConfig) { c.MaxAttempts = 0 }},
		{"negative attempts", func(c *config.KafkaPublisherConfig) { c.MaxAttempts = -1 }},
		{"zero backoff min", func(c *config.KafkaPublisherConfig) { c.WriteBackoffMin = 0 }},
		{"zero backoff max", func(c *config.KafkaPublisherConfig) { c.WriteBackoffMax = 0 }},
		{"inverted backoff", func(c *config.KafkaPublisherConfig) { c.WriteBackoffMin = time.Second }},
		{"unknown compression", func(c *config.KafkaPublisherConfig) { c.Compression = "unknown" }},
		{"missing compression", func(c *config.KafkaPublisherConfig) { c.Compression = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validPublisherConfig()
			tt.change(&cfg)
			writer, err := NewWriter(cfg)
			if writer != nil {
				cleanupWriter(t, writer)
			}
			if err == nil || writer != nil {
				t.Fatalf("NewWriter() = (%v, %v), want configuration rejection", writer, err)
			}
		})
	}
}

// TestNewWriterAcknowledgments protects the three explicit SDK mappings, including no-confirmation publication.
func TestNewWriterAcknowledgments(t *testing.T) {
	tests := []struct {
		policy config.KafkaAcknowledgments
		want   kafkago.RequiredAcks
	}{
		{config.KafkaAcksAll, kafkago.RequireAll},
		{config.KafkaAcksLeader, kafkago.RequireOne},
		{config.KafkaAcksNone, kafkago.RequireNone},
	}
	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			cfg := validPublisherConfig()
			cfg.RequiredAcks = tt.policy
			writer, err := NewWriter(cfg)
			if err != nil {
				t.Fatal(err)
			}
			cleanupWriter(t, writer)
			if writer.writer.RequiredAcks != tt.want {
				t.Fatalf("RequiredAcks = %v, want %v", writer.writer.RequiredAcks, tt.want)
			}
			if writer.writer.Async {
				t.Fatal("acknowledgment choice must not enable async publishing")
			}
		})
	}
}

// TestNewWriterRejectsInvalidAcknowledgments protects direct callers from accidentally falling back to no acknowledgments.
func TestNewWriterRejectsInvalidAcknowledgments(t *testing.T) {
	for _, policy := range []config.KafkaAcknowledgments{"", "unknown", " all "} {
		t.Run(string(policy), func(t *testing.T) {
			cfg := validPublisherConfig()
			cfg.RequiredAcks = policy
			writer, err := NewWriter(cfg)
			if writer != nil {
				cleanupWriter(t, writer)
			}
			if err == nil || writer != nil {
				t.Fatalf("NewWriter() = (%v, %v), want invalid acknowledgment rejection", writer, err)
			}
		})
	}
}
