package observability

import "context"

// Runtime is the observability contract used by app composition.
type Runtime interface {
	LogEmitter() LogEmitter
	Tracer() Tracer
	Meter() Meter
	Shutdown(ctx context.Context) error
}

// NoopRuntime is a no-op implementation of Runtime.
type NoopRuntime struct{}

func (NoopRuntime) LogEmitter() LogEmitter { return NoopLogEmitter{} }

func (NoopRuntime) Tracer() Tracer { return NoopTracer{} }

func (NoopRuntime) Meter() Meter { return NoopMeter{} }

func (NoopRuntime) Shutdown(context.Context) error { return nil }
