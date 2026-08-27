package observabilitymw

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
)

func TestAccessLogAcceptsNilLogger(t *testing.T) {
	NewAccessLog(nil).Record(context.Background(), requestOutcome{})
}

func TestRequestMetricsRecordsBoundedRouteAndError(t *testing.T) {
	meter := observabilitytest.NewRecordingMeter()
	NewRequestMetrics(meter).Record(context.Background(), requestOutcome{
		request:  requestInfo{method: http.MethodGet, route: "/protected", path: "/protected/secret"},
		duration: time.Second,
		status:   http.StatusUnauthorized,
		errorInfo: requestErrorInfo{
			original: errors.New("private error"),
			details:  `{"secret":"value"}`,
		},
	})

	requests := meter.CounterSamples("http_server_requests_total")
	if len(requests) != 1 || requests[0].Fields[keyHTTPRoute] != "/protected" || requests[0].Fields[keyHTTPResponseStatus] != http.StatusUnauthorized {
		t.Fatalf("request metric = %#v", requests)
	}
	if _, exists := requests[0].Fields[keyURLPath]; exists {
		t.Fatalf("request metric contains unbounded URL path: %#v", requests[0].Fields)
	}
	if _, exists := requests[0].Fields[keyErrorDetails]; exists {
		t.Fatalf("request metric contains unbounded error details: %#v", requests[0].Fields)
	}
	if _, exists := requests[0].Fields[keyErrorCode]; exists {
		t.Fatalf("request metric contains application error code: %#v", requests[0].Fields)
	}
	if errorSamples := meter.CounterSamples("http_server_errors_total"); len(errorSamples) != 1 {
		t.Fatalf("error metric count = %d, want 1", len(errorSamples))
	}
	if samples := meter.HistogramSamples("http_server_request_duration_seconds"); len(samples) != 1 {
		t.Fatalf("duration metric count = %d, want 1", len(samples))
	}
}

func TestMiddlewareUsesOneOutcomeForTracingMetricsAndAccessLog(t *testing.T) {
	cause := errors.New("validation dependency failed")
	appErr := apperr.Wrap(cause, apperr.CodeInvalidArgument, "invalid request", apperr.Fields("field", "name"))
	span := &observabilitytest.Span{
		SetAttributesFunc: func(...observability.Fields) {},
		FinishFunc:        func(error, ...string) {},
		IDsFunc:           func() (string, string, bool) { return "trace-01", "span-01", true },
	}
	tracer := &observabilitytest.Tracer{
		ExtractFunc: func(ctx context.Context, _ observability.TextMapCarrier) context.Context { return ctx },
		StartServerFunc: func(ctx context.Context, _, _ string, _ ...observability.Fields) (context.Context, observability.Span) {
			return ctx, span
		},
	}
	meter := observabilitytest.NewRecordingMeter()
	log := &loggertest.Logger{InfoFunc: func(context.Context, string, ...logger.Fields) {}}
	handlerCalls := 0

	e := echo.New()
	e.GET("/protected", New(tracer, log, meter).Handler()(func(c *echo.Context) error {
		handlerCalls++
		traceID, spanID := observability.TraceFromContext(c.Request().Context())
		if traceID != "trace-01" || spanID != "span-01" {
			t.Fatalf("trace context = (%q, %q)", traceID, spanID)
		}
		return httpresponse.NewResponder().AppError(c, appErr)
	}))

	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if handlerCalls != 1 {
		t.Fatalf("handler calls = %d, want 1", handlerCalls)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if tracer.StartServerSpanName != "GET /protected" || tracer.StartServerFields[0][keyHTTPRoute] != "/protected" {
		t.Fatalf("trace start = (%q, %#v)", tracer.StartServerSpanName, tracer.StartServerFields)
	}
	if len(span.FinishCalls) != 1 || !errors.Is(span.FinishCalls[0].Err, cause) {
		t.Fatalf("span finish calls = %#v, want application cause", span.FinishCalls)
	}
	attributes := mergeSpanAttributes(span.SetAttributesCalls)
	if attributes[keyHTTPResponseStatus] != http.StatusBadRequest || attributes[keyErrorDetails] != `{"field":"name"}` {
		t.Fatalf("span completion fields = %#v", attributes)
	}
	if attributes[keyErrorCode] != string(apperr.CodeInvalidArgument) || attributes[keyErrorMessage] != "invalid request" {
		t.Fatalf("span application error fields = %#v", attributes)
	}
	if len(log.InfoCalls) != 1 || len(log.ErrorNoStackCalls) != 0 {
		t.Fatalf("info logs = %d, error logs = %d", len(log.InfoCalls), len(log.ErrorNoStackCalls))
	}
	logFields := log.InfoCalls[0].Fields[0]
	if logFields[keyHTTPRoute] != "/protected" || logFields[keyHTTPResponseStatus] != http.StatusBadRequest || logFields[keyErrorDetails] != `{"field":"name"}` {
		t.Fatalf("access-log fields = %#v", logFields)
	}
	requestSamples := meter.CounterSamples("http_server_requests_total")
	if len(requestSamples) != 1 || requestSamples[0].Fields[keyHTTPRoute] != "/protected" || requestSamples[0].Fields[keyHTTPResponseStatus] != http.StatusBadRequest {
		t.Fatalf("request metrics = %#v", requestSamples)
	}
}

func mergeSpanAttributes(calls []observabilitytest.SetAttributesCall) observability.Fields {
	var sets []observability.Fields
	for _, call := range calls {
		sets = append(sets, call.Fields...)
	}
	return observability.MergeFields(sets...)
}
