package observabilitytest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

type LogEmitCall struct {
	Context  context.Context
	Severity observability.Severity
	Message  string
	Fields   []observability.Fields
}

// LogEmitter is a configurable double for its parent package's LogEmitter contract.
// Configure callbacks before use and inspect calls after concurrent work finishes.
type LogEmitter struct {
	mu        sync.Mutex
	EmitFunc  func(ctx context.Context, severity observability.Severity, message string, fields ...observability.Fields)
	EmitCalls []LogEmitCall
}

func (d *LogEmitter) Emit(ctx context.Context, severity observability.Severity, message string, fields ...observability.Fields) {
	d.mu.Lock()
	d.EmitCalls = append(d.EmitCalls, LogEmitCall{Context: ctx, Severity: severity, Message: message, Fields: append([]observability.Fields(nil), fields...)})
	fn := d.EmitFunc
	d.mu.Unlock()
	if fn == nil {
		panic("unexpected LogEmitter.Emit call")
	}
	fn(ctx, severity, message, fields...)
}
