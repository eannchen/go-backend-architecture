package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/pkg/logger"
	"github.com/eannchen/go-backend-architecture/pkg/logger/loggertest"
)

// TestLogShareAcknowledgmentErrors keeps the template's default policy visible:
// warn with broker metadata and continue, without attributing failure to a batch.
func TestLogShareAcknowledgmentErrors(t *testing.T) {
	log := &loggertest.Logger{WarnFunc: func(context.Context, string, ...logger.Fields) {}}
	failure := errors.New("record lock expired")
	if err := LogShareAcknowledgmentErrors(log, "mail-workers")("mail", 2, failure); err != nil {
		t.Fatalf("default policy requested a stop: %v", err)
	}
	if len(log.WarnCalls) != 1 {
		t.Fatalf("warning calls = %d, want 1", len(log.WarnCalls))
	}
	fields := logger.OptionalFields(log.WarnCalls[0].Fields...)
	if len(fields) != 4 || fields["kafka.group_id"] != "mail-workers" || fields["kafka.topic"] != "mail" || fields["kafka.partition"] != int32(2) || fields["error"] != failure {
		t.Fatalf("unexpected acknowledgment fields: %v", fields)
	}
}
