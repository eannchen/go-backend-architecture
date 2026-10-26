package kafka

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareDecision is the requested outcome for THIS share group. It does not
// delete the original Kafka message or change another group's progress.
type ShareDecision string

const (
	// ShareAccept means handling succeeded. Do not deliver again to this group.
	ShareAccept ShareDecision = "accept"
	// ShareRelease makes the record eligible for another attempt, potentially
	// by another consumer. It can return to this consumer too.
	ShareRelease ShareDecision = "release"
	// ShareReject stops delivery to this group. It does not create a DLT message.
	ShareReject ShareDecision = "reject"
)

var (
	ErrShareBatchStale         = errors.New("share batch is no longer current")
	ErrShareRecordNotInBatch   = errors.New("record does not belong to this share batch")
	ErrShareDecisionAlreadySet = errors.New("share record already has a different decision")
)

// ShareBatch holds one poll's records and tracks whether every record has an
// explicit outcome. Holding it does NOT extend Kafka's record locks. Process
// well before each record's AcquisitionDeadline; DeliveryCount shows how many
// delivery attempts Kafka has made. Both details are available on kgo.Record.
type ShareBatch struct {
	reader    *ShareReader
	records   []*kgo.Record
	decisions map[*kgo.Record]ShareDecision
	pending   int
}

func newShareBatch(reader *ShareReader, records []*kgo.Record) *ShareBatch {
	decisions := make(map[*kgo.Record]ShareDecision, len(records))
	for _, record := range records {
		decisions[record] = ""
	}
	return &ShareBatch{reader: reader, records: records, decisions: decisions, pending: len(records)}
}

// Records copies the slice's array of pointers, not the kgo.Record objects.
// Replacing/reordering entries in the returned slice does not change b.records,
// but both slices still point to the same records (including their payloads).
// Treat the records themselves as read-only:
// use Mark for outcomes, not the SDK's Record.Ack, so the batch can track them.
func (b *ShareBatch) Records() []*kgo.Record { return slices.Clone(b.records) }

// Mark queues one outcome without waiting for Kafka. Only the reader's owner
// calls it, after that record's handler finishes. FlushAcknowledgments reports
// whether Kafka accepted the queued decisions.
// Repeating the same decision is harmless; changing an existing decision fails.
func (b *ShareBatch) Mark(record *kgo.Record, decision ShareDecision) error {
	var status kgo.AckStatus
	switch decision {
	case ShareAccept:
		status = kgo.AckAccept
	case ShareRelease:
		status = kgo.AckRelease
	case ShareReject:
		status = kgo.AckReject
	default:
		return fmt.Errorf("invalid share decision %q", decision)
	}
	r := b.reader
	if r.closed {
		return fmt.Errorf("mark share decision: %w", io.ErrClosedPipe)
	}
	if r.active != b {
		return ErrShareBatchStale
	}
	previous, exists := b.decisions[record]
	if !exists {
		return ErrShareRecordNotInBatch
	}
	if previous == decision {
		return nil
	}
	if previous != "" {
		return ErrShareDecisionAlreadySet
	}
	// Record the decision in the SDK before marking this record as decided.
	r.client.MarkAcks(status, record)
	b.decisions[record] = decision
	b.pending--
	return nil
}
