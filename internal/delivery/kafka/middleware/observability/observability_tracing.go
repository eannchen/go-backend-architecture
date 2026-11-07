package observability

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Tracing struct{ tracer observability.Tracer }

func NewTracing(tracer observability.Tracer) *Tracing {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	return &Tracing{tracer: tracer}
}

func (t *Tracing) Start(ctx context.Context, operation string, record *kgo.Record) (context.Context, observability.Span) {
	return t.tracer.Start(ctx, instrumentationScope, operation, observability.FromPairs(
		keyMessagingSystem, "kafka", keyMessagingDestinationName, record.Topic,
		keyMessagingPartitionID, record.Partition, keyKafkaOffset, record.Offset))
}

func (t *Tracing) Finish(span observability.Span, outcome processingOutcome) {
	span.SetAttributes(observability.FromPairs(keyApplicationKafkaOutcome, outcome.outcome))
	span.Finish(outcome.err)
}
