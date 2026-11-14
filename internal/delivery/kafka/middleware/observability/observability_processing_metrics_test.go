package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestProcessingMetricsAttributes protects bounded metric dimensions and seconds
// as the duration unit, keeping record coordinates and errors in logs/traces.
func TestProcessingMetricsAttributes(t *testing.T) {
	meter := observabilitytest.NewRecordingMeter()
	metrics := NewProcessingMetrics(meter)
	outcome := newProcessingOutcome("activity.consume", &kgo.Record{Topic: "private-topic", Partition: 9, Offset: 99, Key: []byte("private-key")}, 250*time.Millisecond, errors.New("private-error"))
	metrics.Record(context.Background(), outcome)
	attempts := meter.CounterSamples("app.kafka.processing.attempts")
	duration := meter.HistogramSamples("app.kafka.processing.duration")
	if len(attempts) != 1 || attempts[0].Value != 1 || len(duration) != 1 || duration[0].Value != 0.25 {
		t.Fatal("incorrect processing measurements")
	}
	for _, fields := range []map[string]any{attempts[0].Fields, duration[0].Fields} {
		if len(fields) != 2 || fields["app.kafka.operation"] != "activity.consume" || fields["app.kafka.outcome"] != "error" {
			t.Fatal("unexpected metric dimensions")
		}
	}
}
