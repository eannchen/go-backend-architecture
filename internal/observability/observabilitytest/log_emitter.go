package observabilitytest

import (
	"context"
	"sync"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// LogEmitCall records one LogEmitter.Emit invocation.
type LogEmitCall struct {
	Context  context.Context
	Severity observability.Severity
	Message  string
	Fields   []observability.Fields
}

// LogEmitter is the canonical configurable double for observability.LogEmitter.
type LogEmitter struct {
	mu sync.Mutex

	EmitFunc  func(context.Context, observability.Severity, string, ...observability.Fields)
	EmitCalls []LogEmitCall
}

func (e *LogEmitter) Emit(ctx context.Context, severity observability.Severity, message string, fields ...observability.Fields) {
	e.mu.Lock()
	e.EmitCalls = append(e.EmitCalls, LogEmitCall{
		Context:  ctx,
		Severity: severity,
		Message:  message,
		Fields:   append([]observability.Fields(nil), fields...),
	})
	fn := e.EmitFunc
	e.mu.Unlock()
	if fn == nil {
		panic("unexpected LogEmitter.Emit call")
	}
	fn(ctx, severity, message, fields...)
}

var _ observability.LogEmitter = (*LogEmitter)(nil)
