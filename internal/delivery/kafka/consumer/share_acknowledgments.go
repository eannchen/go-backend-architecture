package consumer

import (
	"context"

	"github.com/eannchen/go-backend-architecture/pkg/logger"
)

// LogShareAcknowledgmentErrors returns a callback that warns and continues.
// App wiring can instead supply a callback that returns an error to stop the
// consumer; the lifecycle then logs that failure and applies the sibling policy.
func LogShareAcknowledgmentErrors(log logger.Logger, groupID string) func(topic string, partition int32, err error) error {
	return func(topic string, partition int32, err error) error {
		// SDK results have no individual message context or batch identity.
		log.Warn(context.Background(), "Kafka share acknowledgment failed; delivery may be repeated", logger.FromPairs(
			"kafka.group_id", groupID,
			"kafka.topic", topic,
			"kafka.partition", partition,
			"error", err,
		))
		return nil
	}
}
