package observabilitytest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

// Tracer is a configurable double for its parent package's Tracer contract.
type Tracer struct {
	StartFunc         func(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span)
	StartCalls        []TracerStartCall
	StartServerFunc   func(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span)
	StartServerCalls  []TracerStartServerCall
	StartClientFunc   func(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span)
	StartClientCalls  []TracerStartClientCall
	ExtractFunc       func(ctx context.Context, carrier observability.TextMapCarrier) context.Context
	ExtractCalls      []TracerExtractCall
	InjectFunc        func(ctx context.Context, carrier observability.TextMapCarrier)
	InjectCalls       []TracerInjectCall
	TraceContextFunc  func(ctx context.Context) (observability.TraceContext, bool)
	TraceContextCalls []TracerTraceContextCall
}

// TracerStartCall records one Tracer.Start invocation.
type TracerStartCall struct {
	Context  context.Context
	Scope    string
	SpanName string
	Fields   []observability.Fields
}

func (d *Tracer) Start(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span) {
	d.StartCalls = append(d.StartCalls, TracerStartCall{Context: ctx, Scope: scope, SpanName: spanName, Fields: append([]observability.Fields(nil), fields...)})
	fn := d.StartFunc
	if fn == nil {
		panic("unexpected Tracer.Start call")
	}
	return fn(ctx, scope, spanName, fields...)
}

// TracerStartServerCall records one Tracer.StartServer invocation.
type TracerStartServerCall struct {
	Context  context.Context
	Scope    string
	SpanName string
	Fields   []observability.Fields
}

func (d *Tracer) StartServer(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span) {
	d.StartServerCalls = append(d.StartServerCalls, TracerStartServerCall{Context: ctx, Scope: scope, SpanName: spanName, Fields: append([]observability.Fields(nil), fields...)})
	fn := d.StartServerFunc
	if fn == nil {
		panic("unexpected Tracer.StartServer call")
	}
	return fn(ctx, scope, spanName, fields...)
}

// TracerStartClientCall records one Tracer.StartClient invocation.
type TracerStartClientCall struct {
	Context  context.Context
	Scope    string
	SpanName string
	Fields   []observability.Fields
}

func (d *Tracer) StartClient(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span) {
	d.StartClientCalls = append(d.StartClientCalls, TracerStartClientCall{Context: ctx, Scope: scope, SpanName: spanName, Fields: append([]observability.Fields(nil), fields...)})
	fn := d.StartClientFunc
	if fn == nil {
		panic("unexpected Tracer.StartClient call")
	}
	return fn(ctx, scope, spanName, fields...)
}

// TracerExtractCall records one Tracer.Extract invocation.
type TracerExtractCall struct {
	Context context.Context
	Carrier observability.TextMapCarrier
}

func (d *Tracer) Extract(ctx context.Context, carrier observability.TextMapCarrier) context.Context {
	d.ExtractCalls = append(d.ExtractCalls, TracerExtractCall{Context: ctx, Carrier: carrier})
	fn := d.ExtractFunc
	if fn == nil {
		panic("unexpected Tracer.Extract call")
	}
	return fn(ctx, carrier)
}

// TracerInjectCall records one Tracer.Inject invocation.
type TracerInjectCall struct {
	Context context.Context
	Carrier observability.TextMapCarrier
}

func (d *Tracer) Inject(ctx context.Context, carrier observability.TextMapCarrier) {
	d.InjectCalls = append(d.InjectCalls, TracerInjectCall{Context: ctx, Carrier: carrier})
	fn := d.InjectFunc
	if fn == nil {
		panic("unexpected Tracer.Inject call")
	}
	fn(ctx, carrier)
}

// TracerTraceContextCall records one Tracer.TraceContext invocation.
type TracerTraceContextCall struct {
	Context context.Context
}

func (d *Tracer) TraceContext(ctx context.Context) (observability.TraceContext, bool) {
	d.TraceContextCalls = append(d.TraceContextCalls, TracerTraceContextCall{Context: ctx})
	fn := d.TraceContextFunc
	if fn == nil {
		panic("unexpected Tracer.TraceContext call")
	}
	return fn(ctx)
}
