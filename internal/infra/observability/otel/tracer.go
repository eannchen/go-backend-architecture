package otel

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"vocynex-api/internal/infra/observability"
)

type tracer struct {
	serviceName string
}

type span struct {
	span trace.Span
}

func NewTracer(serviceName string) observability.Tracer {
	return &tracer{serviceName: serviceName}
}

func (t *tracer) Start(ctx context.Context, scope, spanName string, attrs ...[]observability.Field) (context.Context, observability.Span) {
	fields := mergeFieldSets(attrs...)
	opts := []trace.SpanStartOption{}
	if len(fields) > 0 {
		opts = append(opts, trace.WithAttributes(toTraceAttributes(fields)...))
	}
	ctx, s := otel.Tracer(t.tracerName(scope)).Start(ctx, spanName, opts...)
	return ctx, &span{span: s}
}

func (t *tracer) StartServer(ctx context.Context, scope, spanName string, attrs ...[]observability.Field) (context.Context, observability.Span) {
	fields := mergeFieldSets(attrs...)
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindServer),
	}
	if len(fields) > 0 {
		opts = append(opts, trace.WithAttributes(toTraceAttributes(fields)...))
	}
	ctx, s := otel.Tracer(t.tracerName(scope)).Start(ctx, spanName, opts...)
	return ctx, &span{span: s}
}

func (t *tracer) ExtractHTTP(ctx context.Context, headers http.Header) context.Context {
	return propagation.TraceContext{}.Extract(ctx, propagation.HeaderCarrier(headers))
}

func (t *tracer) tracerName(scope string) string {
	return t.serviceName + "/" + scope
}

func (s *span) SetAttributes(attrs ...observability.Field) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(toTraceAttributes(attrs)...)
}

func (s *span) Fail(err error, description string) {
	if s == nil || s.span == nil {
		return
	}
	if err != nil {
		s.span.RecordError(err)
	}
	s.span.SetStatus(codes.Error, description)
}

func (s *span) OK() {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetStatus(codes.Ok, "ok")
}

func (s *span) End() {
	if s == nil || s.span == nil {
		return
	}
	s.span.End()
}

func (s *span) IDs() (traceID, spanID string, ok bool) {
	if s == nil || s.span == nil {
		return "", "", false
	}
	sc := s.span.SpanContext()
	if !sc.IsValid() {
		return "", "", false
	}
	return sc.TraceID().String(), sc.SpanID().String(), true
}

func mergeFieldSets(attrSets ...[]observability.Field) []observability.Field {
	total := 0
	for _, set := range attrSets {
		total += len(set)
	}
	out := make([]observability.Field, 0, total)
	for _, set := range attrSets {
		out = append(out, set...)
	}
	return out
}

func toTraceAttributes(attrs []observability.Field) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, field := range attrs {
		out = append(out, toTraceAttribute(field))
	}
	return out
}

func toTraceAttribute(field observability.Field) attribute.KeyValue {
	switch v := field.Value.(type) {
	case string:
		return attribute.String(field.Key, v)
	case bool:
		return attribute.Bool(field.Key, v)
	case int:
		return attribute.Int64(field.Key, int64(v))
	case int32:
		return attribute.Int64(field.Key, int64(v))
	case int64:
		return attribute.Int64(field.Key, v)
	case uint:
		return attribute.Int64(field.Key, int64(v))
	case uint32:
		return attribute.Int64(field.Key, int64(v))
	case uint64:
		return attribute.Int64(field.Key, int64(v))
	case float32:
		return attribute.Float64(field.Key, float64(v))
	case float64:
		return attribute.Float64(field.Key, v)
	case error:
		return attribute.String(field.Key, v.Error())
	default:
		return attribute.String(field.Key, fmt.Sprintf("%v", field.Value))
	}
}
