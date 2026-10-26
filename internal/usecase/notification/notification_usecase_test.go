package notification

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

// TestHandleLogsOnlyCorrelation verifies the simulation is observable without
// exposing recipients/content and preserves the incoming context for tracing.
func TestHandleLogsOnlyCorrelation(t *testing.T) {
	ctx := context.Background()
	log := &loggertest.Logger{InfoFunc: func(got context.Context, _ string, fields ...logger.Fields) {
		if got != ctx {
			t.Fatal("context replaced")
		}
		if !reflect.DeepEqual(logger.OptionalFields(fields...), logger.Fields{"notification.request_id": "req-1"}) {
			t.Fatalf("unexpected fields: %v", fields)
		}
	}}
	if err := New(log).Handle(ctx, Request{RequestID: "req-1", Recipient: "private-recipient", Content: "private-content"}); err != nil {
		t.Fatal(err)
	}
	if len(log.InfoCalls) != 1 {
		t.Fatal("expected one handled log")
	}
}

// TestHandleCanceled avoids reporting a handled notification when shutdown
// cancellation was already received before the operation started.
func TestHandleCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	log := &loggertest.Logger{}
	if err := New(log).Handle(ctx, Request{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(log.InfoCalls) != 0 {
		t.Fatal("logged canceled handling")
	}
}
