package middleware

import (
	"context"
	"slices"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestChainOrder protects the outer-to-inner declaration used by both apps,
// including the order of completion work after the handler returns.
func TestChainOrder(t *testing.T) {
	var calls []string
	wrap := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, record *kgo.Record) error {
				calls = append(calls, name+" enter")
				err := next(ctx, record)
				calls = append(calls, name+" exit")
				return err
			}
		}
	}
	handler := Chain(func(context.Context, *kgo.Record) error { calls = append(calls, "handle"); return nil }, wrap("context"), wrap("observe"), wrap("recover"))
	if err := handler(t.Context(), &kgo.Record{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"context enter", "observe enter", "recover enter", "handle", "recover exit", "observe exit", "context exit"}
	if !slices.Equal(calls, want) {
		t.Fatalf("order = %v", calls)
	}
}
