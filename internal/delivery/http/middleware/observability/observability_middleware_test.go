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
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type metricSample struct {
	value  int64
	fields observability.Fields
}

type recordingMeter struct {
	counters   map[string][]metricSample
	histograms map[string]int
}

func newRecordingMeter() *recordingMeter {
	return &recordingMeter{counters: make(map[string][]metricSample), histograms: make(map[string]int)}
}

func (m *recordingMeter) Counter(name string, _ ...observability.MetricOption) observability.Counter {
	return recordingCounter{m: m, name: name}
}

func (m *recordingMeter) UpDownCounter(string, ...observability.MetricOption) observability.UpDownCounter {
	return observability.NoopMeter{}.UpDownCounter("")
}

func (m *recordingMeter) Histogram(name string, _ ...observability.MetricOption) observability.Histogram {
	return recordingHistogram{m: m, name: name}
}

type recordingCounter struct {
	m    *recordingMeter
	name string
}

func (c recordingCounter) Add(_ context.Context, value int64, fields ...observability.Fields) {
	c.m.counters[c.name] = append(c.m.counters[c.name], metricSample{value: value, fields: observability.OptionalFields(fields...)})
}

type recordingHistogram struct {
	m    *recordingMeter
	name string
}

func (h recordingHistogram) Record(context.Context, float64, ...observability.Fields) {
	h.m.histograms[h.name]++
}

func TestHTTPContextMetadataReadWrite(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	originalErr := errors.New("boom")
	httpcontext.SetError(c, originalErr)
	httpcontext.SetErrorDetails(c, httpcontext.Details{"stage": "bind"})

	if got := httpcontext.Error(c); got != originalErr {
		t.Fatalf("unexpected original error: %v", got)
	}
	if got := httpcontext.ErrorDetails(c); got == nil || got["stage"] != "bind" {
		t.Fatalf("unexpected error details: %#v", got)
	}
}

func TestAccessLogAcceptsNilLogger(t *testing.T) {
	NewAccessLog(nil).Record(context.Background(), requestOutcome{})
}

func TestRequestMetricsRecordsBoundedRouteAndError(t *testing.T) {
	meter := newRecordingMeter()
	NewRequestMetrics(meter).Record(context.Background(), requestOutcome{
		request:  requestInfo{method: http.MethodGet, route: "/protected", path: "/protected/secret"},
		duration: time.Second,
		status:   http.StatusUnauthorized,
		errorInfo: requestErrorInfo{
			original: errors.New("private error"),
			details:  `{"secret":"value"}`,
		},
	})

	requests := meter.counters["http_server_requests_total"]
	if len(requests) != 1 || requests[0].fields[keyHTTPRoute] != "/protected" || requests[0].fields[keyHTTPResponseStatus] != http.StatusUnauthorized {
		t.Fatalf("request metric = %#v", requests)
	}
	if _, exists := requests[0].fields[keyURLPath]; exists {
		t.Fatalf("request metric contains unbounded URL path: %#v", requests[0].fields)
	}
	if _, exists := requests[0].fields[keyErrorDetails]; exists {
		t.Fatalf("request metric contains unbounded error details: %#v", requests[0].fields)
	}
	if _, exists := requests[0].fields[keyErrorCode]; exists {
		t.Fatalf("request metric contains application error code: %#v", requests[0].fields)
	}
	if errors := meter.counters["http_server_errors_total"]; len(errors) != 1 {
		t.Fatalf("error metric count = %d, want 1", len(errors))
	}
	if got := meter.histograms["http_server_request_duration_seconds"]; got != 1 {
		t.Fatalf("latency metric count = %d, want 1", got)
	}
}

func TestMiddlewareUsesOneOutcomeForTracingMetricsAndAccessLog(t *testing.T) {
	cause := errors.New("validation dependency failed")
	appErr := apperr.Wrap(cause, apperr.CodeInvalidArgument, "invalid request", apperr.Fields("field", "name"))
	span := &recordingSpan{traceID: "trace-01", spanID: "span-01"}
	tracer := &recordingTracer{span: span}
	meter := newRecordingMeter()
	log := &loggertest.Logger{}
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
	if tracer.startName != "GET /protected" || tracer.startFields[keyHTTPRoute] != "/protected" {
		t.Fatalf("trace start = (%q, %#v)", tracer.startName, tracer.startFields)
	}
	if !errors.Is(span.finishErr, cause) {
		t.Fatalf("span finish error = %v, want application cause", span.finishErr)
	}
	if span.attributes[keyHTTPResponseStatus] != http.StatusBadRequest || span.attributes[keyErrorDetails] != `{"field":"name"}` {
		t.Fatalf("span completion fields = %#v", span.attributes)
	}
	if span.attributes[keyErrorCode] != string(apperr.CodeInvalidArgument) || span.attributes[keyErrorMessage] != "invalid request" {
		t.Fatalf("span application error fields = %#v", span.attributes)
	}
	if log.InfoCalls != 1 || log.ErrorNoStackCalls != 0 {
		t.Fatalf("info logs = %d, error logs = %d", log.InfoCalls, log.ErrorNoStackCalls)
	}
	logFields := log.Infos[0].Fields[0]
	if logFields[keyHTTPRoute] != "/protected" || logFields[keyHTTPResponseStatus] != http.StatusBadRequest || logFields[keyErrorDetails] != `{"field":"name"}` {
		t.Fatalf("access-log fields = %#v", logFields)
	}
	requestSamples := meter.counters["http_server_requests_total"]
	if len(requestSamples) != 1 || requestSamples[0].fields[keyHTTPRoute] != "/protected" || requestSamples[0].fields[keyHTTPResponseStatus] != http.StatusBadRequest {
		t.Fatalf("request metrics = %#v", requestSamples)
	}
}

type recordingTracer struct {
	span        *recordingSpan
	startName   string
	startFields observability.Fields
}

func (t *recordingTracer) Start(ctx context.Context, _, _ string, _ ...observability.Fields) (context.Context, observability.Span) {
	return ctx, t.span
}

func (t *recordingTracer) StartServer(ctx context.Context, _, name string, fields ...observability.Fields) (context.Context, observability.Span) {
	t.startName = name
	t.startFields = observability.OptionalFields(fields...)
	return ctx, t.span
}

func (*recordingTracer) Extract(ctx context.Context, _ observability.TextMapCarrier) context.Context {
	return ctx
}

type recordingSpan struct {
	traceID    string
	spanID     string
	attributes observability.Fields
	finishErr  error
}

func (s *recordingSpan) SetAttributes(fields ...observability.Fields) {
	s.attributes = observability.MergeFields(s.attributes, observability.OptionalFields(fields...))
}

func (s *recordingSpan) Finish(err error, _ ...string) { s.finishErr = err }

func (s *recordingSpan) IDs() (string, string, bool) {
	return s.traceID, s.spanID, s.traceID != "" && s.spanID != ""
}
