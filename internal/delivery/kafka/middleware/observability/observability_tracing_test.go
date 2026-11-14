package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	contracts "github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestProcessingTracing protects the processing span context and failure
// completion, while excluding message keys and payloads from span attributes.
func TestProcessingTracing(t *testing.T) {
	type key struct{}
	span := &observabilitytest.Span{
		SetAttributesFunc: func(...contracts.Fields) {},
		FinishFunc:        func(error, ...string) {},
	}
	tracer := &observabilitytest.Tracer{StartFunc: func(ctx context.Context, scope, name string, _ ...contracts.Fields) (context.Context, contracts.Span) {
		if scope != "delivery.kafka" || name != "activity.consume" {
			t.Error("wrong span identity")
		}
		return context.WithValue(ctx, key{}, "processing"), span
	}}
	failure := errors.New("workflow failed")
	handler := New(tracer, nil, nil).Handler("activity.consume")(func(ctx context.Context, _ *kgo.Record) error {
		if ctx.Value(key{}) != "processing" {
			t.Error("span context not passed to handler")
		}
		return failure
	})
	err := handler(t.Context(), &kgo.Record{Topic: "activities", Partition: 2, Offset: 7, Key: []byte("private-key"), Value: []byte("private-body")})
	if !errors.Is(err, failure) || tracer.StartCalls != 1 || len(span.FinishCalls) != 1 || !errors.Is(span.FinishCalls[0].Err, failure) {
		t.Fatal("span lifecycle lost processing failure")
	}
	if len(span.SetAttributesCalls) != 1 || span.SetAttributesCalls[0].Fields[0]["app.kafka.outcome"] != "error" {
		t.Fatal("span outcome missing")
	}
	if strings.Contains(fmt.Sprint(tracer.StartFields), "private-") {
		t.Fatal("private data included in span")
	}
}
