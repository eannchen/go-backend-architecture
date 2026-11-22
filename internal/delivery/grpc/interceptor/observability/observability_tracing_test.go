package observability

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appobservability "github.com/eannchen/go-backend-architecture/pkg/observability"
	"github.com/eannchen/go-backend-architecture/pkg/observability/observabilitytest"
)

// TestTracingFinishUsesServerStatus keeps server response classification in the
// interceptor, including plain gRPC errors without application-specific methods.
func TestTracingFinishUsesServerStatus(t *testing.T) {
	for _, tt := range []struct {
		code   codes.Code
		failed bool
	}{
		{code: codes.OK},
		{code: codes.InvalidArgument},
		{code: codes.Canceled},
		{code: codes.ResourceExhausted},
		{code: codes.Internal, failed: true},
		{code: codes.Unavailable, failed: true},
		{code: codes.DeadlineExceeded, failed: true},
	} {
		t.Run(tt.code.String(), func(t *testing.T) {
			err := status.Error(tt.code, "RPC outcome")
			span := &observabilitytest.Span{
				SetAttributesFunc: func(...appobservability.Fields) {},
				FinishFunc:        func(appobservability.SpanOutcome) {},
			}
			NewTracing(appobservability.NoopTracer{}).Finish(span, newRPCOutcome(rpcInfo{}, 0, err))
			if len(span.FinishCalls) != 1 {
				t.Fatalf("finish calls = %d, want 1", len(span.FinishCalls))
			}
			outcome := span.FinishCalls[0].Outcome
			if outcome.Err != err || outcome.Failed != tt.failed {
				t.Fatalf("outcome = %+v, want error %v and failed %v", outcome, err, tt.failed)
			}
		})
	}
}
