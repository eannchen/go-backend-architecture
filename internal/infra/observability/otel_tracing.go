package observability

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type otelTracer struct {
	serviceName string
}

type otelSpan struct {
	span trace.Span
}

func NewTracer(serviceName string) Tracer {
	return &otelTracer{serviceName: serviceName}
}

func (t *otelTracer) tracerName(scope string) string {
	return t.serviceName + "/" + scope
}

func (t *otelTracer) Start(ctx context.Context, scope, spanName string, attrs ...KV) (context.Context, Span) {
	opts := []trace.SpanStartOption{}
	if len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(toTraceAttributes(attrs...)...))
	}
	ctx, s := otel.Tracer(t.tracerName(scope)).Start(ctx, spanName, opts...)
	return ctx, &otelSpan{span: s}
}

func (t *otelTracer) StartServer(ctx context.Context, scope, spanName string, attrs ...KV) (context.Context, Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindServer),
	}
	if len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(toTraceAttributes(attrs...)...))
	}
	ctx, s := otel.Tracer(t.tracerName(scope)).Start(ctx, spanName, opts...)
	return ctx, &otelSpan{span: s}
}

func (s *otelSpan) SetAttributes(attrs ...KV) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(toTraceAttributes(attrs...)...)
}

func (s *otelSpan) Fail(err error, description string) {
	if s == nil || s.span == nil {
		return
	}
	if err != nil {
		s.span.RecordError(err)
	}
	s.span.SetStatus(codes.Error, description)
}

func (s *otelSpan) OK() {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetStatus(codes.Ok, "ok")
}

func (s *otelSpan) End() {
	if s == nil || s.span == nil {
		return
	}
	s.span.End()
}

func (s *otelSpan) IDs() (traceID, spanID string, ok bool) {
	if s == nil || s.span == nil {
		return "", "", false
	}
	sc := s.span.SpanContext()
	if !sc.IsValid() {
		return "", "", false
	}
	return sc.TraceID().String(), sc.SpanID().String(), true
}

func (t *otelTracer) ExtractHTTP(ctx context.Context, headers http.Header) context.Context {
	return propagation.TraceContext{}.Extract(ctx, propagation.HeaderCarrier(headers))
}

func toTraceAttributes(attrs ...KV) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		out = append(out, toTraceAttribute(kv))
	}
	return out
}

func toTraceAttribute(kv KV) attribute.KeyValue {
	switch v := kv.Value.(type) {
	case string:
		return attribute.String(kv.Key, v)
	case bool:
		return attribute.Bool(kv.Key, v)
	case int:
		return attribute.Int64(kv.Key, int64(v))
	case int32:
		return attribute.Int64(kv.Key, int64(v))
	case int64:
		return attribute.Int64(kv.Key, v)
	case uint:
		return attribute.Int64(kv.Key, int64(v))
	case uint32:
		return attribute.Int64(kv.Key, int64(v))
	case uint64:
		return attribute.Int64(kv.Key, int64(v))
	case float32:
		return attribute.Float64(kv.Key, float64(v))
	case float64:
		return attribute.Float64(kv.Key, v)
	case error:
		return attribute.String(kv.Key, v.Error())
	default:
		return attribute.String(kv.Key, fmt.Sprintf("%v", kv.Value))
	}
}
