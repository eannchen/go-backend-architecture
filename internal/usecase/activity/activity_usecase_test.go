package activity

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

// TestHandleActivity protects cancellation and privacy of the simulated activity
// side effect. Only the event correlation ID belongs in the business log.
func TestHandleActivity(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		if canceled {
			cancel()
		}
		log := &loggertest.Logger{InfoFunc: func(got context.Context, _ string, fields ...logger.Fields) {
			if got != ctx {
				t.Error("context replaced")
			}
			values := logger.OptionalFields(fields...)
			if len(values) != 1 || values["activity.event_id"] != "evt-1" {
				t.Error("private activity data logged")
			}
		}}
		err := New(log).Handle(ctx, Request{EventID: "evt-1", Action: "private-action"})
		cancel()
		if canceled {
			if !errors.Is(err, context.Canceled) || len(log.InfoCalls) != 0 {
				t.Fatal("canceled activity reported as handled")
			}
		} else if err != nil || len(log.InfoCalls) != 1 {
			t.Fatal("activity handling not recorded")
		}
	}
}
