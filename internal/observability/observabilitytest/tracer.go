package observabilitytest

import (
	"context"
	"net/http"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Tracer is the canonical configurable double for observability.Tracer.
type Tracer struct {
	StartFunc           func(context.Context, string, string, ...observability.Fields) (context.Context, observability.Span)
	StartCalls          int
	StartScope          string
	StartSpanName       string
	StartFields         []observability.Fields
	StartServerFunc     func(context.Context, string, string, ...observability.Fields) (context.Context, observability.Span)
	StartServerCalls    int
	StartServerScope    string
	StartServerSpanName string
	StartServerFields   []observability.Fields
	ExtractHTTPFunc     func(context.Context, http.Header) context.Context
	ExtractHTTPCalls    int
	ExtractHTTPHeaders  http.Header
}

func (t *Tracer) Start(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span) {
	t.StartCalls++
	t.StartScope = scope
	t.StartSpanName = spanName
	t.StartFields = fields
	if t.StartFunc == nil {
		panic("unexpected Tracer.Start call")
	}
	return t.StartFunc(ctx, scope, spanName, fields...)
}

func (t *Tracer) StartServer(ctx context.Context, scope, spanName string, fields ...observability.Fields) (context.Context, observability.Span) {
	t.StartServerCalls++
	t.StartServerScope = scope
	t.StartServerSpanName = spanName
	t.StartServerFields = fields
	if t.StartServerFunc == nil {
		panic("unexpected Tracer.StartServer call")
	}
	return t.StartServerFunc(ctx, scope, spanName, fields...)
}

func (t *Tracer) ExtractHTTP(ctx context.Context, headers http.Header) context.Context {
	t.ExtractHTTPCalls++
	t.ExtractHTTPHeaders = headers
	if t.ExtractHTTPFunc == nil {
		panic("unexpected Tracer.ExtractHTTP call")
	}
	return t.ExtractHTTPFunc(ctx, headers)
}

// Span is the canonical configurable double for observability.Span.
type Span struct {
	SetAttributesFunc  func(...observability.Fields)
	SetAttributesCalls []SetAttributesCall
	FinishFunc         func(error, ...string)
	FinishCalls        []FinishCall
	IDsFunc            func() (string, string, bool)
	IDsCalls           int
}

// SetAttributesCall records one Span.SetAttributes invocation.
type SetAttributesCall struct {
	Fields []observability.Fields
}

// FinishCall records one Span.Finish invocation.
type FinishCall struct {
	Err         error
	Description []string
}

func (s *Span) SetAttributes(fields ...observability.Fields) {
	s.SetAttributesCalls = append(s.SetAttributesCalls, SetAttributesCall{Fields: fields})
	if s.SetAttributesFunc == nil {
		panic("unexpected Span.SetAttributes call")
	}
	s.SetAttributesFunc(fields...)
}

func (s *Span) Finish(err error, description ...string) {
	s.FinishCalls = append(s.FinishCalls, FinishCall{Err: err, Description: description})
	if s.FinishFunc == nil {
		panic("unexpected Span.Finish call")
	}
	s.FinishFunc(err, description...)
}

func (s *Span) IDs() (string, string, bool) {
	s.IDsCalls++
	if s.IDsFunc == nil {
		panic("unexpected Span.IDs call")
	}
	return s.IDsFunc()
}

var _ observability.Tracer = (*Tracer)(nil)
var _ observability.Span = (*Span)(nil)
