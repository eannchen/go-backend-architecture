package observabilitymw

import (
	"errors"
	"net/http"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/pkg/observability"
	"github.com/eannchen/go-backend-architecture/pkg/observability/observabilitytest"
)

// TestTracingFinishUsesResponseStatus ensures server span failure follows the
// final HTTP response, independently of the presence or type of a Go error.
func TestTracingFinishUsesResponseStatus(t *testing.T) {
	cause := errors.New("request rejected")
	for _, tt := range []struct {
		name   string
		status int
		err    error
		failed bool
	}{
		{name: "success", status: http.StatusOK},
		{name: "client response with ordinary error", status: http.StatusBadRequest, err: cause},
		{name: "server response with application client error", status: http.StatusInternalServerError, err: apperr.New(apperr.CodeInvalidArgument, "invalid input"), failed: true},
		{name: "server response without Go error", status: http.StatusServiceUnavailable, failed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			span := &observabilitytest.Span{
				SetAttributesFunc: func(...observability.Fields) {},
				FinishFunc:        func(observability.SpanOutcome) {},
			}
			NewTracing(observability.NoopTracer{}).Finish(span, requestOutcome{
				responseStatusCode: tt.status,
				applicationError:   applicationErrorInfo{originalError: tt.err},
			})
			if len(span.FinishCalls) != 1 {
				t.Fatalf("finish calls = %d, want 1", len(span.FinishCalls))
			}
			outcome := span.FinishCalls[0].Outcome
			if outcome.Err != tt.err || outcome.Failed != tt.failed {
				t.Fatalf("outcome = %+v, want error %v and failed %v", outcome, tt.err, tt.failed)
			}
		})
	}
}
