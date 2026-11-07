package observability

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// AccessLog writes one completion log for each record processing attempt.
// The worker commits or acknowledges afterward, so success means handling finished.
type AccessLog struct{ log logger.Logger }

func NewAccessLog(log logger.Logger) *AccessLog {
	if log == nil {
		log = logger.NoopLogger{}
	}
	return &AccessLog{log: log}
}

// Record uses the processing context so the logger can attach trace IDs.
// Keys, payloads, and headers are excluded because they can contain private data.
func (l *AccessLog) Record(ctx context.Context, outcome processingOutcome) {
	fields := logger.FromPairs(
		keyMessagingSystem, "kafka",
		keyMessagingDestinationName, outcome.record.Topic,
		keyMessagingPartitionID, outcome.record.Partition,
		keyKafkaOffset, outcome.record.Offset,
		keyApplicationKafkaOperation, outcome.operation,
		keyApplicationKafkaOutcome, outcome.outcome,
		keyLogDurationMS, outcome.duration.Milliseconds(),
	)
	if outcome.err != nil {
		fields[keyErrorType] = outcome.outcome
	}
	switch outcome.outcome {
	case "invalid":
		l.log.Warn(ctx, "Kafka record processing completed", fields)
	case "error", "timeout":
		l.log.ErrorNoStack(ctx, "Kafka record processing completed", outcome.err, fields)
	default:
		// Recovery already logged the panic stack. Cancellation is expected during
		// shutdown or when another handler stops a Share Group batch.
		l.log.Info(ctx, "Kafka record processing completed", fields)
	}
}
