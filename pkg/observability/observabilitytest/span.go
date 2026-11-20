package observabilitytest

import (
	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

type SetAttributesCall struct {
	Fields []observability.Fields
}

type FinishCall struct {
	Err         error
	Description []string
}

// Span is a configurable double for its parent package's Span contract.
type Span struct {
	SetAttributesFunc  func(fields ...observability.Fields)
	SetAttributesCalls []SetAttributesCall
	FinishFunc         func(err error, description ...string)
	FinishCalls        []FinishCall
}

func (d *Span) SetAttributes(fields ...observability.Fields) {
	d.SetAttributesCalls = append(d.SetAttributesCalls, SetAttributesCall{Fields: append([]observability.Fields(nil), fields...)})
	fn := d.SetAttributesFunc
	if fn == nil {
		panic("unexpected Span.SetAttributes call")
	}
	fn(fields...)
}

func (d *Span) Finish(err error, description ...string) {
	d.FinishCalls = append(d.FinishCalls, FinishCall{Err: err, Description: append([]string(nil), description...)})
	fn := d.FinishFunc
	if fn == nil {
		panic("unexpected Span.Finish call")
	}
	fn(err, description...)
}
