//go:build integration

package kafka

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestKafkaRoundTrip protects actual publication, codecs, key ordering, header
// preservation, and explicit commit behavior against the pinned broker image.
func TestKafkaRoundTrip(t *testing.T) {
	for _, codec := range []config.KafkaCompression{config.CompressionNone, config.CompressionGzip, config.CompressionSnappy, config.CompressionLz4, config.CompressionZstd} {
		t.Run(string(codec), func(t *testing.T) {
			topic := createIntegrationTopic(t, 3)
			group := topic + "-group"
			cleanupIntegrationGroup(t, group)
			cfg := validPublisherConfig()
			cfg.Brokers = integrationBrokers
			cfg.Topic = topic
			cfg.PartitionStrategy = config.PartitionByKey
			cfg.Compression = codec
			writer, err := NewWriter(cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			cleanupWriter(t, writer)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			records := []*kgo.Record{
				{Key: []byte("user-1"), Value: []byte("first"), Headers: []kgo.RecordHeader{{Key: "correlation", Value: []byte("req-1")}}},
				{Key: []byte("user-1"), Value: []byte("second")},
			}
			if err := writer.WriteRecords(ctx, records...); err != nil {
				t.Fatal(err)
			}
			reader := newIntegrationReader(t, topic, group)
			for _, want := range records {
				got, err := reader.FetchRecord(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if got.Topic != topic || got.Partition != want.Partition || got.Offset != want.Offset || !reflect.DeepEqual(got.Key, want.Key) || !reflect.DeepEqual(got.Value, want.Value) || !reflect.DeepEqual(got.Headers, want.Headers) {
					t.Fatalf("roundtrip differs: got=%+v want=%+v", got, want)
				}
				if err := reader.CommitRecords(ctx, got); err != nil {
					t.Fatal(err)
				}
				reader.AllowRebalance()
			}
			if records[0].Partition != records[1].Partition || records[0].Offset >= records[1].Offset {
				t.Fatal("key ordering lost")
			}
		})
	}
}

// TestKafkaWeakerAcknowledgments confirms configurable leader/none policies
// remain usable after the migration disables idempotency for those modes.
func TestKafkaWeakerAcknowledgments(t *testing.T) {
	for _, acks := range []config.KafkaAcknowledgments{config.KafkaAcksLeader, config.KafkaAcksNone} {
		t.Run(string(acks), func(t *testing.T) {
			topic := createIntegrationTopic(t, 1)
			group := topic + "-group"
			cleanupIntegrationGroup(t, group)
			cfg := validPublisherConfig()
			cfg.Brokers = integrationBrokers
			cfg.Topic = topic
			cfg.RequiredAcks = acks
			writer, err := NewWriter(cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			cleanupWriter(t, writer)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte("accepted")}); err != nil {
				t.Fatal(err)
			}
			reader := newIntegrationReader(t, topic, group)
			record, err := reader.FetchRecord(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if string(record.Value) != "accepted" {
				t.Fatal("record differs")
			}
			if err := reader.CommitRecords(ctx, record); err != nil {
				t.Fatal(err)
			}
			reader.AllowRebalance()
		})
	}
}

func newIntegrationWriter(t *testing.T, topic string) *Writer {
	t.Helper()
	cfg := validPublisherConfig()
	cfg.Brokers = integrationBrokers
	cfg.Topic = topic
	writer, err := NewWriter(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanupWriter(t, writer)
	return writer
}
