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

// TestActivityHandler protects workflow invocation and error identity.
// Decoder edge cases live beside the message decoder.
func TestActivityHandler(t *testing.T) {
	failure := errors.New("workflow failed")
	for _, tc := range []struct {
		name, payload string
		result, want  error
		calls         int
	}{
		{"success", `{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}`, nil, nil, 1},
		{"failure", `{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}`, failure, failure, 1},
		{"invalid", "{}", nil, middleware.ErrInvalidMessage, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &activitytest.Usecase{HandleFunc: func(ctx context.Context, request activity.Request) error {
				if ctx != t.Context() {
					t.Fatal("context changed")
				}
				want := activity.Request{EventID: "evt-1", Action: "report.downloaded", OccurredAt: time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)}
				if request != want {
					t.Fatal("request mapping changed")
				}
				return tc.result
			}}
			err := NewActivityHandler(usecase)(t.Context(), &kgo.Record{Value: []byte(tc.payload)})
			if !errors.Is(err, tc.want) || len(usecase.HandleCalls) != tc.calls {
				t.Fatalf("result = %v, calls = %d", err, len(usecase.HandleCalls))
			}
		})
	}
}
