package observability

import (
	"context"
	"errors"
	"io"
	"testing"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
)

func TestUnaryInjectsTraceAndRecordsOutcome(t *testing.T) {
	span := newTestSpan()
	tracer := &observabilitytest.Tracer{
		StartClientFunc: func(ctx context.Context, _, _ string, _ ...appobservability.Fields) (context.Context, appobservability.Span) {
			return ctx, span
		},
		InjectFunc: func(_ context.Context, carrier appobservability.TextMapCarrier) {
			carrier.Set("traceparent", "propagated")
		},
	}
	log := &loggertest.Logger{InfoFunc: func(context.Context, string, ...logger.Fields) {}}
	meter := observabilitytest.NewRecordingMeter()

	err := New(
		Config{DependencyName: "diagnostics", Target: "diagnostics:9090"},
		WithTracing(tracer, TraceConfig{PropagateContext: true}),
		WithMetrics(meter),
		WithCompletionLog(log, fixedLogPolicy(logger.SeverityInfo)),
	).Unary()(context.Background(), "/diagnostics.v1.DiagnosticsService/GetHealth", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *googlegrpc.ClientConn, _ ...googlegrpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		if md.Get("traceparent")[0] != "propagated" {
			t.Fatalf("outgoing metadata = %#v", md)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unary() error = %v", err)
	}
	if tracer.StartClientCalls != 1 || tracer.InjectCalls != 1 {
		t.Fatalf("tracer calls = start:%d inject:%d", tracer.StartClientCalls, tracer.InjectCalls)
	}
	if len(log.InfoCalls) != 1 || len(log.ErrorNoStackCalls) != 0 {
		t.Fatalf("log calls = info:%d error:%d", len(log.InfoCalls), len(log.ErrorNoStackCalls))
	}
	requests := meter.CounterSamples("grpc_client_requests_total")
	if len(requests) != 1 || requests[0].Fields[keyRPCMethod] != "diagnostics.v1.DiagnosticsService/GetHealth" || requests[0].Fields[keyRPCResponseStatusCode] != "OK" || requests[0].Fields[keyApplicationDependencyName] != "diagnostics" {
		t.Fatalf("request metrics = %#v", requests)
	}
	if requests[0].Fields[keyServerAddress] != "diagnostics" || requests[0].Fields[keyServerPort] != 9090 {
		t.Fatalf("request metric destination = %#v", requests[0].Fields)
	}
	if len(span.FinishCalls) != 1 || span.FinishCalls[0].Err != nil {
		t.Fatalf("span finish calls = %#v", span.FinishCalls)
	}
}

func TestUnaryRecordsRemoteFailure(t *testing.T) {
	wireErr := status.Error(codes.Unavailable, "dependency unavailable")
	span := newTestSpan()
	tracer := &observabilitytest.Tracer{
		StartClientFunc: func(ctx context.Context, _, _ string, _ ...appobservability.Fields) (context.Context, appobservability.Span) {
			return ctx, span
		},
		InjectFunc: func(context.Context, appobservability.TextMapCarrier) {},
	}
	log := &loggertest.Logger{ErrorNoStackFunc: func(context.Context, string, error, ...logger.Fields) {}}
	meter := observabilitytest.NewRecordingMeter()

	var loggedOutcome LogOutcome
	err := New(
		Config{DependencyName: "external-provider"},
		WithTracing(tracer, TraceConfig{}),
		WithMetrics(meter),
		WithCompletionLog(log, func(outcome LogOutcome) (logger.Severity, bool) {
			loggedOutcome = outcome
			return logger.SeverityError, true
		}),
	).Unary()(context.Background(), "/test.Service/Fail", nil, nil, nil, func(context.Context, string, any, any, *googlegrpc.ClientConn, ...googlegrpc.CallOption) error {
		return wireErr
	})
	if !errors.Is(err, wireErr) {
		t.Fatalf("Unary() error = %v, want wire error", err)
	}
	if tracer.InjectCalls != 0 {
		t.Fatalf("Inject() calls = %d, want no external trace propagation", tracer.InjectCalls)
	}
	if len(log.ErrorNoStackCalls) != 1 || !errors.Is(log.ErrorNoStackCalls[0].Err, wireErr) {
		t.Fatalf("error logs = %#v", log.ErrorNoStackCalls)
	}
	if loggedOutcome.DependencyName != "external-provider" || loggedOutcome.RPCMethod != "test.Service/Fail" || loggedOutcome.GRPCStatusCode != codes.Unavailable || !errors.Is(loggedOutcome.RPCError, wireErr) {
		t.Fatalf("log policy outcome = %+v", loggedOutcome)
	}
	if samples := meter.CounterSamples("grpc_client_errors_total"); len(samples) != 1 || samples[0].Fields[keyRPCResponseStatusCode] != "UNAVAILABLE" {
		t.Fatalf("error metrics = %#v", samples)
	}
}

func TestStreamFinishesOnEOF(t *testing.T) {
	span := newTestSpan()
	tracer := &observabilitytest.Tracer{
		StartClientFunc: func(ctx context.Context, _, _ string, _ ...appobservability.Fields) (context.Context, appobservability.Span) {
			return ctx, span
		},
		InjectFunc: func(context.Context, appobservability.TextMapCarrier) {},
	}
	log := &loggertest.Logger{InfoFunc: func(context.Context, string, ...logger.Fields) {}}
	meter := observabilitytest.NewRecordingMeter()
	base := &testClientStream{recvErr: io.EOF}

	stream, err := New(
		Config{},
		WithTracing(tracer, TraceConfig{}),
		WithMetrics(meter),
		WithCompletionLog(log, fixedLogPolicy(logger.SeverityInfo)),
	).Stream()(context.Background(), &googlegrpc.StreamDesc{ServerStreams: true}, nil, "/grpc.health.v1.Health/Watch", func(context.Context, *googlegrpc.StreamDesc, *googlegrpc.ClientConn, string, ...googlegrpc.CallOption) (googlegrpc.ClientStream, error) {
		return base, nil
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if err := stream.RecvMsg(nil); err != io.EOF {
		t.Fatalf("RecvMsg() error = %v, want EOF", err)
	}
	active := meter.UpDownCounterSamples("grpc_client_active_streams")
	if len(active) != 2 || active[0].Value != 1 || active[1].Value != -1 {
		t.Fatalf("active stream metrics = %#v", active)
	}
	if len(span.FinishCalls) != 1 || span.FinishCalls[0].Err != nil {
		t.Fatalf("span finish calls = %#v", span.FinishCalls)
	}
}

func TestUnaryCapabilitiesAreOptIn(t *testing.T) {
	wantErr := status.Error(codes.NotFound, "missing")
	invokerCalls := 0

	err := New(Config{DependencyName: "unused"}).Unary()(context.Background(), "/test.Service/Get", nil, nil, nil, func(context.Context, string, any, any, *googlegrpc.ClientConn, ...googlegrpc.CallOption) error {
		invokerCalls++
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("Unary() error = %v, want original error", err)
	}
	if invokerCalls != 1 {
		t.Fatalf("invoker calls = %d, want 1", invokerCalls)
	}
}

func TestUnaryCapabilitiesCanBeSelectedIndependently(t *testing.T) {
	t.Run("tracing only", func(t *testing.T) {
		span := newTestSpan()
		tracer := &observabilitytest.Tracer{
			StartClientFunc: func(ctx context.Context, _, _ string, _ ...appobservability.Fields) (context.Context, appobservability.Span) {
				return ctx, span
			},
		}

		err := New(Config{}, WithTracing(tracer, TraceConfig{})).Unary()(context.Background(), "/test.Service/Get", nil, nil, nil, successfulInvoker)

		if err != nil {
			t.Fatalf("Unary() error = %v", err)
		}
		if tracer.StartClientCalls != 1 || tracer.InjectCalls != 0 || len(span.FinishCalls) != 1 {
			t.Fatalf("tracing calls = start:%d inject:%d finish:%d", tracer.StartClientCalls, tracer.InjectCalls, len(span.FinishCalls))
		}
	})

	t.Run("metrics only", func(t *testing.T) {
		meter := observabilitytest.NewRecordingMeter()

		err := New(Config{}, WithMetrics(meter)).Unary()(context.Background(), "/test.Service/Get", nil, nil, nil, successfulInvoker)

		if err != nil {
			t.Fatalf("Unary() error = %v", err)
		}
		if samples := meter.CounterSamples("grpc_client_requests_total"); len(samples) != 1 {
			t.Fatalf("request metric samples = %#v, want one", samples)
		}
	})
}

func TestCompletionLogPolicyCanSkipAnOutcome(t *testing.T) {
	log := &loggertest.Logger{
		DebugFunc:        func(context.Context, string, ...logger.Fields) {},
		InfoFunc:         func(context.Context, string, ...logger.Fields) {},
		WarnFunc:         func(context.Context, string, ...logger.Fields) {},
		ErrorNoStackFunc: func(context.Context, string, error, ...logger.Fields) {},
	}

	err := New(Config{}, WithCompletionLog(log, func(LogOutcome) (logger.Severity, bool) {
		return logger.SeverityInfo, false
	})).Unary()(context.Background(), "/test.Service/Get", nil, nil, nil, func(context.Context, string, any, any, *googlegrpc.ClientConn, ...googlegrpc.CallOption) error {
		return nil
	})

	if err != nil {
		t.Fatalf("Unary() error = %v", err)
	}
	if len(log.DebugCalls)+len(log.InfoCalls)+len(log.WarnCalls)+len(log.ErrorNoStackCalls) != 0 {
		t.Fatalf("log calls = debug:%d info:%d warn:%d error:%d, want none", len(log.DebugCalls), len(log.InfoCalls), len(log.WarnCalls), len(log.ErrorNoStackCalls))
	}
}

func fixedLogPolicy(severity logger.Severity) LogPolicy {
	return func(LogOutcome) (logger.Severity, bool) {
		return severity, true
	}
}

func successfulInvoker(context.Context, string, any, any, *googlegrpc.ClientConn, ...googlegrpc.CallOption) error {
	return nil
}

func newTestSpan() *observabilitytest.Span {
	return &observabilitytest.Span{
		SetAttributesFunc: func(...appobservability.Fields) {},
		FinishFunc:        func(error, ...string) {},
	}
}

type testClientStream struct {
	googlegrpc.ClientStream
	recvErr error
}

func (s *testClientStream) RecvMsg(any) error { return s.recvErr }
