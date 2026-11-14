package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification/notificationtest"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestNotificationHandler protects workflow invocation and error identity.
// Decoder edge cases live beside the message decoder.
func TestNotificationHandler(t *testing.T) {
	failure := errors.New("workflow failed")
	for _, tc := range []struct {
		name, payload string
		result, want  error
		calls         int
	}{
		{"success", `{"request_id":"req-1","recipient":"user-1","content":"Ready"}`, nil, nil, 1},
		{"failure", `{"request_id":"req-1","recipient":"user-1","content":"Ready"}`, failure, failure, 1},
		{"invalid", "{}", nil, middleware.ErrInvalidMessage, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &notificationtest.Usecase{HandleFunc: func(ctx context.Context, request notification.Request) error {
				if ctx != t.Context() {
					t.Fatal("context changed")
				}
				want := notification.Request{RequestID: "req-1", Recipient: "user-1", Content: "Ready"}
				if request != want {
					t.Fatal("request mapping changed")
				}
				return tc.result
			}}
			err := NewNotificationHandler(usecase)(t.Context(), &kgo.Record{Value: []byte(tc.payload)})
			if !errors.Is(err, tc.want) || len(usecase.HandleCalls) != tc.calls {
				t.Fatalf("result = %v, calls = %d", err, len(usecase.HandleCalls))
			}
		})
	}
}
