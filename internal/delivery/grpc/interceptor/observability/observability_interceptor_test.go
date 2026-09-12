package observability

import (
	"context"
	"errors"
	"testing"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestUnaryRecordsTraceLogAndMetrics(t *testing.T) {
	tracer := &recordingTracer{span: &recordingSpan{}}
	meter := newRecordingMeter()
	log := &loggertest.Logger{InfoFunc: func(context.Context, string, ...logger.Fields) {}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"))

	_, err := New(tracer, log, meter).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{FullMethod: "/diagnostics.v1.DiagnosticsService/GetHealth"}, func(ctx context.Context, _ any) (any, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if !tracer.extractCalled || tracer.startName != "diagnostics.v1.DiagnosticsService/GetHealth" {
		t.Fatalf("tracer state = %#v", tracer)
	}
	if tracer.span.finishErr != nil {
		t.Fatalf("span finish error = %v, want nil", tracer.span.finishErr)
	}
	if len(log.InfoCalls) != 1 || len(log.ErrorNoStackCalls) != 0 {
		t.Fatalf("info logs = %d, error logs = %d", len(log.InfoCalls), len(log.ErrorNoStackCalls))
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
	log := &loggertest.Logger{ErrorNoStackFunc: func(context.Context, string, error, ...logger.Fields) {}}
	wireErr := grpcresponse.NewResponder().AppError(cause)

	_, gotErr := New(tracer, log, meter).Unary()(context.Background(), nil, &googlegrpc.UnaryServerInfo{FullMethod: "/test.Service/Fail"}, func(context.Context, any) (any, error) {
		return nil, wireErr
	})

	if gotErr != wireErr {
		t.Fatalf("returned error = %v, want original wire error", gotErr)
	}
	if len(log.ErrorNoStackCalls) != 1 || !errors.Is(log.ErrorNoStackCalls[0].Err, cause) {
		t.Fatalf("error logs = %#v, want original cause", log.ErrorNoStackCalls)
	}
	if got := meter.counterValues("grpc_server_errors_total"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("error samples = %v", got)
	}
	if got := tracer.span.attributes[keyApplicationErrorCauseChain]; got != cause.Error() {
		t.Fatalf("span error chain = %v, want %q", got, cause.Error())
	}
	if got := tracer.span.attributes[keyRPCResponseStatusCode]; got != "INTERNAL" {
		t.Fatalf("span gRPC status = %v, want INTERNAL", got)
	}
	if got := tracer.span.attributes[keyErrorType]; got != "INTERNAL" {
		t.Fatalf("span error type = %v, want INTERNAL", got)
	}
	if _, exists := tracer.span.attributes[keyApplicationErrorCode]; exists {
		t.Fatalf("span has application code for non-application error: %#v", tracer.span.attributes)
	}
	if got := tracer.span.attributes[keyApplicationErrorMessage]; got != "internal server error" {
		t.Fatalf("span error message = %v, want internal server error", got)
	}
	fields := log.ErrorNoStackCalls[0].Fields[0]
	if fields[keyApplicationErrorCauseChain] != cause.Error() || fields[keyRPCResponseStatusCode] != "INTERNAL" {
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
	log := &loggertest.Logger{InfoFunc: func(context.Context, string, ...logger.Fields) {}}
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
	if got := tracer.span.attributes[keyApplicationErrorCauseChain]; got != appErr.Error()+"; "+cause.Error() {
		t.Fatalf("span error chain = %v", got)
	}
	if got := tracer.span.attributes[keyApplicationErrorDetails]; got != `{"field":"name"}` {
		t.Fatalf("span error details = %v", got)
	}
	if got := tracer.span.attributes[keyRPCResponseStatusCode]; got != "INVALID_ARGUMENT" {
		t.Fatalf("span gRPC status = %v", got)
	}
	if got := tracer.span.attributes[keyApplicationErrorCode]; got != string(apperr.CodeInvalidArgument) {
		t.Fatalf("span application error code = %v", got)
	}
	if got := tracer.span.attributes[keyApplicationErrorMessage]; got != "invalid request" {
		t.Fatalf("span error message = %v", got)
	}
	if _, exists := tracer.span.attributes[keyErrorType]; exists {
		t.Fatalf("span treats a client-caused server status as a server failure: %#v", tracer.span.attributes)
	}
	if len(log.InfoCalls) != 1 || len(log.ErrorNoStackCalls) != 0 {
		t.Fatalf("info logs = %d, error logs = %d", len(log.InfoCalls), len(log.ErrorNoStackCalls))
	}
	fields := log.InfoCalls[0].Fields[0]
	if fields[keyApplicationErrorDetails] != `{"field":"name"}` || fields[keyApplicationErrorCode] != string(apperr.CodeInvalidArgument) || fields[keyRPCResponseStatusCode] != "INVALID_ARGUMENT" {
		t.Fatalf("access-log error fields = %#v", fields)
	}
	for _, sample := range meter.counters["grpc_server_errors_total"] {
		if sample.fields[keyRPCResponseStatusCode] != "INVALID_ARGUMENT" {
			t.Fatalf("error metric gRPC status = %#v", sample.fields)
		}
		if _, exists := sample.fields[keyApplicationErrorDetails]; exists {
			t.Fatalf("error metric contains unbounded details field: %#v", sample.fields)
		}
		if _, exists := sample.fields[keyApplicationErrorCode]; exists {
			t.Fatalf("error metric contains application error code: %#v", sample.fields)
		}
	}
}

func TestStreamWrapsContextAndTracksActiveStream(t *testing.T) {
	tracer := &recordingTracer{span: &recordingSpan{}}
	meter := newRecordingMeter()
	base := observabilityServerStream{ctx: context.Background()}

	err := New(tracer, nil, meter).Stream()(nil, base, &googlegrpc.StreamServerInfo{
		FullMethod:     "/grpc.health.v1.Health/Watch",
		IsServerStream: true,
	}, func(_ any, stream googlegrpc.ServerStream) error {
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
func (t *recordingTracer) StartClient(ctx context.Context, _, _ string, _ ...appobservability.Fields) (context.Context, appobservability.Span) {
	return ctx, t.span
}
func (t *recordingTracer) Extract(ctx context.Context, carrier appobservability.TextMapCarrier) context.Context {
	t.extractCalled = carrier.Get("traceparent") != ""
	return ctx
}
func (*recordingTracer) Inject(context.Context, appobservability.TextMapCarrier) {}
func (*recordingTracer) TraceContext(context.Context) (appobservability.TraceContext, bool) {
	return appobservability.TraceContext{}, false
}

type recordingSpan struct {
	attributes appobservability.Fields
	finishErr  error
}

func (s *recordingSpan) SetAttributes(fields ...appobservability.Fields) {
	s.attributes = appobservability.MergeFields(s.attributes, appobservability.OptionalFields(fields...))
}
func (s *recordingSpan) Finish(err error, _ ...string) { s.finishErr = err }

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
