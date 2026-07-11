package otel

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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
	fields := observability.OptionalFields(optionalFields...)
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
