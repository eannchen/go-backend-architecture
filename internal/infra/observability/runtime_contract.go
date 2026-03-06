package observability

import "context"

// Runtime is the observability facade exposed to app wiring.
type Runtime interface {
	LogEmitter() LogEmitter
	Tracer() Tracer
	Shutdown(ctx context.Context) error
}

// NoopRuntime is a no-op observability facade.
type NoopRuntime struct{}

func (NoopRuntime) LogEmitter() LogEmitter { return NoopLogEmitter{} }

func (NoopRuntime) Tracer() Tracer { return NoopTracer{} }

func (NoopRuntime) Shutdown(context.Context) error { return nil }
