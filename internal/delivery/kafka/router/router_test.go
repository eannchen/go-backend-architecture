package router

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TestRoutesSelectOneHandler protects topic isolation, context propagation, and
// error identity so the worker can make the correct commit/acknowledgment decision.
func TestRoutesSelectOneHandler(t *testing.T) {
	ctx := t.Context()
	failure := errors.New("activity failed")
	calls := make(map[string]int)
	routes := map[string]middleware.Handler{}
	for _, topic := range []string{"notifications", "activities"} {
		routes[topic] = func(got context.Context, record *kgo.Record) error {
			if got != ctx || record.Topic != topic {
				t.Fatal("incorrect dispatch")
			}
			calls[topic]++
			if topic == "activities" {
				return failure
			}
			return nil
		}
	}
	handler, err := New([]string{"notifications", "activities"}, routes)
	if err != nil {
		t.Fatal(err)
	}
	// Construction freezes the registrations so callers cannot change live routes.
	delete(routes, "notifications")
	if err := handler(ctx, &kgo.Record{Topic: "notifications"}); err != nil {
		t.Fatal(err)
	}
	if err := handler(ctx, &kgo.Record{Topic: "activities"}); !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	if calls["notifications"] != 1 || calls["activities"] != 1 {
		t.Fatalf("calls = %v", calls)
	}
	if err := handler(ctx, &kgo.Record{Topic: "unknown"}); err == nil {
		t.Fatal("unknown topic succeeded")
	}
}

// TestMissingRouteFailsAtConstruction prevents a worker from starting with
// subscriptions that have no usable handler.
func TestMissingRouteFailsAtConstruction(t *testing.T) {
	for _, routes := range []map[string]middleware.Handler{nil, {"notifications": nil}} {
		if _, err := New([]string{"notifications"}, routes); err == nil {
			t.Fatal("missing handler accepted")
		}
	}
}
