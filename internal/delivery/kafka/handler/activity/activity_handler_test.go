package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/usecase/activity"
	"github.com/eannchen/go-backend-architecture/internal/usecase/activity/activitytest"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestActivityHandler validates its own payload and preserves workflow failures,
// preventing malformed records from reaching business processing.
func TestActivityHandler(t *testing.T) {
	valid := `{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}`
	for _, tc := range []struct {
		name, payload string
		valid         bool
	}{
		{"valid", valid, true},
		{"syntax", "{", false},
		{"null", "null", false},
		{"notification payload", `{"request_id":"req-1","recipient":"user-1","content":"ready"}`, false},
		{"missing action", `{"event_id":"evt-1","occurred_at":"2026-11-12T09:00:00Z"}`, false},
		{"invalid time", `{"event_id":"evt-1","action":"download","occurred_at":"yesterday"}`, false},
		{"multiple objects", valid + valid, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := errors.New("processing failed")
			usecase := &activitytest.Usecase{HandleFunc: func(ctx context.Context, request activity.Request) error {
				if ctx != t.Context() || request.EventID != "evt-1" || request.Action != "report.downloaded" || !request.OccurredAt.Equal(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)) {
					t.Fatalf("incorrect mapping: %+v", request)
				}
				return failure
			}}
			err := NewActivityHandler(usecase)(t.Context(), &kgo.Record{Value: []byte(tc.payload)})
			if tc.valid {
				if !errors.Is(err, failure) || len(usecase.HandleCalls) != 1 {
					t.Fatalf("workflow result: %v", err)
				}
			} else if !errors.Is(err, middleware.ErrInvalidMessage) || len(usecase.HandleCalls) != 0 {
				t.Fatalf("invalid payload reached workflow: %v", err)
			}
		})
	}
}
