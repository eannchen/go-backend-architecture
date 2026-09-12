package otel

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

type tracer struct {
	serviceName string
}

type clientErrorReporter interface {
	IsClientError() bool
}

func NewTracer(serviceName string) observability.Tracer {
	return &tracer{serviceName: serviceName}
}

func (t *tracer) Start(ctx context.Context, scope, spanName string, optionalFields ...observability.Fields) (context.Context, observability.Span) {
	fields := observability.OptionalFields(optionalFields...)
	opts := []trace.SpanStartOption{}
	if len(fields) > 0 {
		opts = append(opts, trace.WithAttributes(toTraceAttributes(fields)...))
	}
	ctx, s := otel.Tracer(t.tracerName(scope)).Start(ctx, spanName, opts...)
	return ctx, &span{span: s}
}

func (t *tracer) StartServer(ctx context.Context, scope, spanName string, optionalFields ...observability.Fields) (context.Context, observability.Span) {
	return t.startWithKind(ctx, scope, spanName, trace.SpanKindServer, optionalFields...)
}

func (t *tracer) StartClient(ctx context.Context, scope, spanName string, optionalFields ...observability.Fields) (context.Context, observability.Span) {
	return t.startWithKind(ctx, scope, spanName, trace.SpanKindClient, optionalFields...)
}

func (t *tracer) startWithKind(ctx context.Context, scope, spanName string, kind trace.SpanKind, optionalFields ...observability.Fields) (context.Context, observability.Span) {
	fields := observability.OptionalFields(optionalFields...)
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(kind),
	}
	if len(fields) > 0 {
		opts = append(opts, trace.WithAttributes(toTraceAttributes(fields)...))
	}
	// Start always creates a new span. When ctx contains a SpanContext extracted
	// from an upstream trace, the new span keeps its trace ID and uses the
	// upstream span ID as its parent. Otherwise, OTel starts a new root trace.
	// The returned context stores the active span in OTel's structured form; it
	// does not yet contain literal traceparent or tracestate transport fields.
	ctx, s := otel.Tracer(t.tracerName(scope)).Start(ctx, spanName, opts...)
	return ctx, &span{span: s}
}

func (t *tracer) Extract(ctx context.Context, carrier observability.TextMapCarrier) context.Context {
	if carrier == nil {
		return ctx
	}
	// A server transport adapter exposes incoming headers or gRPC metadata
	// through carrier. TraceContext decodes traceparent and tracestate into a
	// remote SpanContext stored in the returned context. Extract does not start
	// a span; the following StartServer call uses that remote span as its parent.
	// Missing or invalid trace metadata leaves no usable remote parent, so
	// StartServer naturally begins a new trace instead of rejecting the request.
	return propagation.TraceContext{}.Extract(ctx, textMapCarrier{carrier})
}

func (t *tracer) Inject(ctx context.Context, carrier observability.TextMapCarrier) {
	if carrier == nil {
		return
	}
	// A client span is already stored in ctx. This is where its structured
	// SpanContext becomes literal wire fields: TraceContext encodes traceparent,
	// plus tracestate when present, into the carrier. The transport adapter then
	// sends those fields as HTTP headers or gRPC metadata.
	propagation.TraceContext{}.Inject(ctx, textMapCarrier{carrier})
}

func (*tracer) TraceContext(ctx context.Context) (observability.TraceContext, bool) {
	// Read the native span context at log time so stdout uses the same IDs as
	// propagation and OTLP export, without mirroring them into custom context keys.
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return observability.TraceContext{}, false
	}
	return observability.TraceContext{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
	}, true
}

type textMapCarrier struct{ observability.TextMapCarrier }

var _ propagation.TextMapCarrier = textMapCarrier{}

func (t *tracer) tracerName(scope string) string {
	return t.serviceName + "/" + scope
}

type span struct {
	span trace.Span
}

func (s *span) SetAttributes(optionalFields ...observability.Fields) {
	if s == nil || s.span == nil {
		return
	}
	fields := observability.OptionalFields(optionalFields...)
	s.span.SetAttributes(toTraceAttributes(fields)...)
}

func (s *span) Finish(err error, description ...string) {
	if s == nil || s.span == nil {
		return
	}

	if err != nil {
		s.span.RecordError(err)
		if isClientError(err) {
			s.span.SetStatus(codes.Ok, "")
		} else {
			desc := err.Error()
			if len(description) > 0 && description[0] != "" {
				desc = description[0]
			}
			s.span.SetStatus(codes.Error, desc)
		}
	} else {
		s.span.SetStatus(codes.Ok, "")
	}
	s.span.End()
}

func isClientError(err error) bool {
	var reporter clientErrorReporter
	return errors.As(err, &reporter) && reporter.IsClientError()
}

func toTraceAttributes(fields observability.Fields) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(fields))
	for key, value := range fields {
		out = append(out, toTraceAttribute(key, value))
	}
	return out
}

func toTraceAttribute(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case bool:
		return attribute.Bool(key, v)
	case int:
		return attribute.Int64(key, int64(v))
	case int32:
		return attribute.Int64(key, int64(v))
	case int64:
		return attribute.Int64(key, v)
	case uint:
		return attribute.Int64(key, int64(v))
	case uint32:
		return attribute.Int64(key, int64(v))
	case uint64:
		return attribute.Int64(key, int64(v))
	case float32:
		return attribute.Float64(key, float64(v))
	case float64:
		return attribute.Float64(key, v)
	case error:
		return attribute.String(key, v.Error())
	default:
		return attribute.String(key, fmt.Sprintf("%v", value))
	}
}
