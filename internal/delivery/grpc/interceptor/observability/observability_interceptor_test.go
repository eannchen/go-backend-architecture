package observability

import (
	"context"
	"errors"
	"testing"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestUnaryRecordsTraceLogAndMetrics(t *testing.T) {
	tracer := &recordingTracer{span: &recordingSpan{traceID: "trace-01", spanID: "span-01"}}
	meter := newRecordingMeter()
	log := &loggertest.Logger{}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"))

	_, err := New(tracer, log, meter).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{FullMethod: "/diagnostics.v1.DiagnosticsService/GetHealth"}, func(ctx context.Context, _ any) (any, error) {
		traceID, spanID := appobservability.TraceFromContext(ctx)
		if traceID != "trace-01" || spanID != "span-01" {
			t.Fatalf("trace context = (%q, %q)", traceID, spanID)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if !tracer.extractCalled || tracer.startName != "/diagnostics.v1.DiagnosticsService/GetHealth" {
		t.Fatalf("tracer state = %#v", tracer)
	}
	if tracer.span.finishErr != nil {
		t.Fatalf("span finish error = %v, want nil", tracer.span.finishErr)
	}
	if log.InfoCalls != 1 || log.ErrorNoStackCalls != 0 {
		t.Fatalf("info logs = %d, error logs = %d", log.InfoCalls, log.ErrorNoStackCalls)
	}
	if got := meter.counterValues("grpc_server_requests_total"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("request samples = %v", got)
	}
	if got := meter.counterValues("grpc_server_errors_total"); len(got) != 0 {
		t.Fatalf("error samples = %v, want none", got)
	}
	if got := meter.histogramCalls["grpc_server_request_duration_seconds"]; got != 1 {
		t.Fatalf("duration samples = %d, want 1", got)
	}
}

func TestUnaryRecordsOriginalServerFailure(t *testing.T) {
	cause := errors.New("database connection failed")
	tracer := &recordingTracer{span: &recordingSpan{}}
	meter := newRecordingMeter()
	log := &loggertest.Logger{}
	wireErr := grpcresponse.NewResponder().AppError(cause)

	_, gotErr := New(tracer, log, meter).Unary()(context.Background(), nil, &googlegrpc.UnaryServerInfo{FullMethod: "/test.Service/Fail"}, func(context.Context, any) (any, error) {
		return nil, wireErr
	})

	if gotErr != wireErr {
		t.Fatalf("returned error = %v, want original wire error", gotErr)
	}
	if log.ErrorNoStackCalls != 1 || !errors.Is(log.ErrorsNoStack[0].Err, cause) {
		t.Fatalf("error logs = %#v, want original cause", log.ErrorsNoStack)
	}
	if got := meter.counterValues("grpc_server_errors_total"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("error samples = %v", got)
	}
	if got := tracer.span.attributes[keyError]; got != cause.Error() {
		t.Fatalf("span error = %v, want %q", got, cause.Error())
	}
	if got := tracer.span.attributes[keyErrorChain]; got != cause.Error() {
		t.Fatalf("span error chain = %v, want %q", got, cause.Error())
	}
	if got := tracer.span.attributes[keyTransportCode]; got != codes.Internal.String() {
		t.Fatalf("span transport code = %v, want %q", got, codes.Internal.String())
	}
	if got := tracer.span.attributes[keyTransportMessage]; got != "internal server error" {
		t.Fatalf("span transport message = %v, want internal server error", got)
	}
	fields := log.ErrorsNoStack[0].Fields[0]
	if fields[keyErrorChain] != cause.Error() || fields[keyTransportCode] != codes.Internal.String() {
		t.Fatalf("access-log error fields = %#v", fields)
	}
}

func TestUnaryRecordsApplicationErrorDetailsWithoutAddingThemToMetrics(t *testing.T) {
	cause := errors.New("name lookup failed")
	appErr := apperr.Wrap(
		cause,
		apperr.CodeInvalidArgument,
		"invalid request",
		apperr.Fields("field", "name"),
	)
	tracer := &recordingTracer{span: &recordingSpan{}}
	meter := newRecordingMeter()
	log := &loggertest.Logger{}
	wireErr := grpcresponse.NewResponder().AppError(appErr)

	_, gotErr := New(tracer, log, meter).Unary()(context.Background(), nil, &googlegrpc.UnaryServerInfo{FullMethod: "/test.Service/Validate"}, func(context.Context, any) (any, error) {
		return nil, wireErr
	})

	if gotErr != wireErr {
		t.Fatalf("returned error = %v, want original wire error", gotErr)
	}
	if tracer.span.finishErr != wireErr {
		t.Fatalf("span finish error = %v, want wire error", tracer.span.finishErr)
	}
	if got := tracer.span.attributes[keyError]; got != appErr.Error() {
		t.Fatalf("span error = %v, want %q", got, appErr.Error())
	}
	if got := tracer.span.attributes[keyErrorChain]; got != appErr.Error()+"; "+cause.Error() {
		t.Fatalf("span error chain = %v", got)
	}
	if got := tracer.span.attributes[keyErrorDetails]; got != `{"field":"name"}` {
		t.Fatalf("span error details = %v", got)
	}
	if got := tracer.span.attributes[keyTransportCode]; got != codes.InvalidArgument.String() {
		t.Fatalf("span transport code = %v", got)
	}
	if got := tracer.span.attributes[keyTransportMessage]; got != "invalid request" {
		t.Fatalf("span transport message = %v", got)
	}
	if log.InfoCalls != 1 || log.ErrorNoStackCalls != 0 {
		t.Fatalf("info logs = %d, error logs = %d", log.InfoCalls, log.ErrorNoStackCalls)
	}
	fields := log.Infos[0].Fields[0]
	if fields[keyErrorDetails] != `{"field":"name"}` || fields[keyTransportCode] != codes.InvalidArgument.String() {
		t.Fatalf("access-log error fields = %#v", fields)
	}
	for _, sample := range meter.counters["grpc_server_errors_total"] {
		if _, exists := sample.fields[keyError]; exists {
			t.Fatalf("error metric contains unbounded error field: %#v", sample.fields)
		}
		if _, exists := sample.fields[keyErrorDetails]; exists {
			t.Fatalf("error metric contains unbounded details field: %#v", sample.fields)
		}
	}
}

func TestStreamWrapsContextAndTracksActiveStream(t *testing.T) {
	tracer := &recordingTracer{span: &recordingSpan{traceID: "trace-02", spanID: "span-02"}}
	meter := newRecordingMeter()
	base := observabilityServerStream{ctx: context.Background()}

	err := New(tracer, nil, meter).Stream()(nil, base, &googlegrpc.StreamServerInfo{
		FullMethod:     "/grpc.health.v1.Health/Watch",
		IsServerStream: true,
	}, func(_ any, stream googlegrpc.ServerStream) error {
		traceID, spanID := appobservability.TraceFromContext(stream.Context())
		if traceID != "trace-02" || spanID != "span-02" {
			t.Fatalf("trace context = (%q, %q)", traceID, spanID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got := meter.upDownValues("grpc_server_active_streams"); len(got) != 2 || got[0] != 1 || got[1] != -1 {
		t.Fatalf("active stream samples = %v, want [1 -1]", got)
	}
}

type recordingTracer struct {
	span          *recordingSpan
	extractCalled bool
	startName     string
}

func (t *recordingTracer) Start(ctx context.Context, _, _ string, _ ...appobservability.Fields) (context.Context, appobservability.Span) {
	return ctx, t.span
}
func (t *recordingTracer) StartServer(ctx context.Context, _, name string, _ ...appobservability.Fields) (context.Context, appobservability.Span) {
	t.startName = name
	return ctx, t.span
}
func (t *recordingTracer) Extract(ctx context.Context, carrier appobservability.TextMapCarrier) context.Context {
	t.extractCalled = carrier.Get("traceparent") != ""
	return ctx
}

type recordingSpan struct {
	traceID    string
	spanID     string
	attributes appobservability.Fields
	finishErr  error
}

func (s *recordingSpan) SetAttributes(fields ...appobservability.Fields) {
	s.attributes = appobservability.MergeFields(s.attributes, appobservability.OptionalFields(fields...))
}
func (s *recordingSpan) Finish(err error, _ ...string) { s.finishErr = err }
func (s *recordingSpan) IDs() (string, string, bool) {
	return s.traceID, s.spanID, s.traceID != "" && s.spanID != ""
}

type metricSample struct {
	value  int64
	fields appobservability.Fields
}

type recordingMeter struct {
	counters       map[string][]metricSample
	upDownCounters map[string][]metricSample
	histogramCalls map[string]int
}

func newRecordingMeter() *recordingMeter {
	return &recordingMeter{
		counters:       make(map[string][]metricSample),
		upDownCounters: make(map[string][]metricSample),
		histogramCalls: make(map[string]int),
	}
}
func (m *recordingMeter) Counter(name string, _ ...appobservability.MetricOption) appobservability.Counter {
	return recordingCounter{samples: m.counters, name: name}
}
func (m *recordingMeter) UpDownCounter(name string, _ ...appobservability.MetricOption) appobservability.UpDownCounter {
	return recordingCounter{samples: m.upDownCounters, name: name}
}
func (m *recordingMeter) Histogram(name string, _ ...appobservability.MetricOption) appobservability.Histogram {
	return recordingHistogram{calls: m.histogramCalls, name: name}
}
func (m *recordingMeter) counterValues(name string) []int64 {
	return sampleValues(m.counters[name])
}
func (m *recordingMeter) upDownValues(name string) []int64 {
	return sampleValues(m.upDownCounters[name])
}

type recordingCounter struct {
	samples map[string][]metricSample
	name    string
}

func (c recordingCounter) Add(_ context.Context, value int64, fields ...appobservability.Fields) {
	c.samples[c.name] = append(c.samples[c.name], metricSample{value: value, fields: appobservability.OptionalFields(fields...)})
}

type recordingHistogram struct {
	calls map[string]int
	name  string
}

func (h recordingHistogram) Record(context.Context, float64, ...appobservability.Fields) {
	h.calls[h.name]++
}

func sampleValues(samples []metricSample) []int64 {
	values := make([]int64, len(samples))
	for index, sample := range samples {
		values[index] = sample.value
	}
	return values
}

type observabilityServerStream struct{ ctx context.Context }

func (observabilityServerStream) SetHeader(metadata.MD) error  { return nil }
func (observabilityServerStream) SendHeader(metadata.MD) error { return nil }
func (observabilityServerStream) SetTrailer(metadata.MD)       {}
func (s observabilityServerStream) Context() context.Context   { return s.ctx }
func (observabilityServerStream) SendMsg(any) error            { return nil }
func (observabilityServerStream) RecvMsg(any) error            { return nil }
