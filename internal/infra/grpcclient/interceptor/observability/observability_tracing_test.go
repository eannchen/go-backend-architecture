package observability

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appobservability "github.com/eannchen/go-backend-architecture/pkg/observability"
	"github.com/eannchen/go-backend-architecture/pkg/observability/observabilitytest"
)

// TestTracingFinishUsesClientStatus treats every non-OK RPC as a failed outbound
// call, even when the same status would not fail a server span.
func TestTracingFinishUsesClientStatus(t *testing.T) {
	for _, code := range []codes.Code{codes.OK, codes.InvalidArgument, codes.Canceled, codes.Unavailable} {
		t.Run(code.String(), func(t *testing.T) {
			err := status.Error(code, "remote response")
			span := &observabilitytest.Span{
				SetAttributesFunc: func(...appobservability.Fields) {},
				FinishFunc:        func(appobservability.SpanOutcome) {},
			}
			NewTracing(appobservability.NoopTracer{}, TraceConfig{}).Finish(span, newRPCOutcome(rpcInfo{}, 0, err))
			if len(span.FinishCalls) != 1 {
				t.Fatalf("finish calls = %d, want 1", len(span.FinishCalls))
			}
			outcome := span.FinishCalls[0].Outcome
			if outcome.Err != err || outcome.Failed != (code != codes.OK) || outcome.Description != status.Convert(err).Message() {
				t.Fatalf("unexpected outcome for %s: %+v", code, outcome)
			}
		})
	}
}
