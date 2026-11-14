//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware/context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestShareAcquisitionDeadlineUsesBrokerMetadata verifies the deadline against
// a genuinely acquired record. Synthetic ordinary records have no Kafka lock.
func TestShareAcquisitionDeadlineUsesBrokerMetadata(t *testing.T) {
	runtime := newTelemetry(t)
	topic := backend.CreateTopic(t, 1)
	group := topic + "-deadline"
	prepareGroup(t, true, group)
	flow := newWorkflow(t, runtime, true, topic, group, func(context.Context, *kgo.Record) error { return nil })
	writer := newPublisher(t, runtime, topic)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	if err := writer.WriteRecords(ctx, &kgo.Record{Value: []byte(notificationJSON)}); err != nil {
		t.Fatal(err)
	}
	record, err := flow.next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if record.AcquisitionDeadline().IsZero() {
		t.Fatal("broker acquisition metadata missing")
	}
	margin := 2 * time.Second
	called := false
	handler := contextmw.New(runtime.Tracer(), contextmw.Config{HandlingTimeout: time.Minute, AcknowledgmentMargin: margin}).Handler()(func(ctx context.Context, _ *kgo.Record) error {
		called = true
		deadline, ok := ctx.Deadline()
		if !ok || !deadline.Equal(record.AcquisitionDeadline().Add(-margin)) {
			t.Error("handler deadline ignored the acquisition budget")
		}
		return nil
	})
	// Start processing with an unconstrained parent so setup latency cannot
	// make the fetch timeout earlier than the acquisition budget under test.
	if err := handler(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler not invoked")
	}
	if err := flow.complete(ctx, record); err != nil {
		t.Fatal(err)
	}
}
