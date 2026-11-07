package observability

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type ProcessingMetrics struct {
	attempts observability.Counter
	duration observability.Histogram
}

func NewProcessingMetrics(meter observability.Meter) *ProcessingMetrics {
	if meter == nil {
		meter = observability.NoopMeter{}
	}
	return &ProcessingMetrics{
		attempts: meter.Counter("app.kafka.processing.attempts", observability.MetricOption{Description: "Completed record processing attempts", Unit: "{attempt}"}),
		duration: meter.Histogram("app.kafka.processing.duration", observability.MetricOption{Description: "Record processing duration", Unit: "s"}),
	}
}

func (m *ProcessingMetrics) Record(ctx context.Context, outcome processingOutcome) {
	// Operations come from app wiring. IDs, record coordinates, and raw errors
	// belong in logs and spans because they would create unbounded metric labels.
	fields := observability.FromPairs(keyApplicationKafkaOperation, outcome.operation, keyApplicationKafkaOutcome, outcome.outcome)
	m.attempts.Add(ctx, 1, fields)
	m.duration.Record(ctx, outcome.duration.Seconds(), fields)
}
