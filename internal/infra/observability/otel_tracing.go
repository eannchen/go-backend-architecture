package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Span is a small wrapper to centralize common OTel span operations.
type Span struct {
	span trace.Span
}

func StartSpan(ctx context.Context, tracerName, spanName string, attrs ...attribute.KeyValue) (context.Context, *Span) {
	opts := []trace.SpanStartOption{}
	if len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(attrs...))
	}
	ctx, s := otel.Tracer(tracerName).Start(ctx, spanName, opts...)
	return ctx, &Span{span: s}
}

func StartSpanWithOptions(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, *Span) {
	ctx, s := otel.Tracer(tracerName).Start(ctx, spanName, opts...)
	return ctx, &Span{span: s}
}

func (s *Span) SetAttributes(attrs ...attribute.KeyValue) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(attrs...)
}

func (s *Span) Fail(err error, description string) {
	if s == nil || s.span == nil {
		return
	}
	if err != nil {
		s.span.RecordError(err)
	}
	s.span.SetStatus(codes.Error, description)
}

func (s *Span) OK() {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetStatus(codes.Ok, "ok")
}

func (s *Span) End() {
	if s == nil || s.span == nil {
		return
	}
	s.span.End()
}

func (s *Span) SpanContext() trace.SpanContext {
	if s == nil || s.span == nil {
		return trace.SpanContext{}
	}
	return s.span.SpanContext()
}
