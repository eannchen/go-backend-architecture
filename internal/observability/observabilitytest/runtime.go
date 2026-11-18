package observabilitytest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Runtime is a configurable double for its parent package's Runtime contract.
type Runtime struct {
	LogEmitterFunc  func() observability.LogEmitter
	LogEmitterCalls []RuntimeLogEmitterCall
	TracerFunc      func() observability.Tracer
	TracerCalls     []RuntimeTracerCall
	MeterFunc       func() observability.Meter
	MeterCalls      []RuntimeMeterCall
	ShutdownFunc    func(ctx context.Context) error
	ShutdownCalls   []RuntimeShutdownCall
}

// RuntimeLogEmitterCall records one Runtime.LogEmitter invocation.
type RuntimeLogEmitterCall struct {
}

func (d *Runtime) LogEmitter() observability.LogEmitter {
	d.LogEmitterCalls = append(d.LogEmitterCalls, RuntimeLogEmitterCall{})
	fn := d.LogEmitterFunc
	if fn == nil {
		panic("unexpected Runtime.LogEmitter call")
	}
	return fn()
}

// RuntimeTracerCall records one Runtime.Tracer invocation.
type RuntimeTracerCall struct {
}

func (d *Runtime) Tracer() observability.Tracer {
	d.TracerCalls = append(d.TracerCalls, RuntimeTracerCall{})
	fn := d.TracerFunc
	if fn == nil {
		panic("unexpected Runtime.Tracer call")
	}
	return fn()
}

// RuntimeMeterCall records one Runtime.Meter invocation.
type RuntimeMeterCall struct {
}

func (d *Runtime) Meter() observability.Meter {
	d.MeterCalls = append(d.MeterCalls, RuntimeMeterCall{})
	fn := d.MeterFunc
	if fn == nil {
		panic("unexpected Runtime.Meter call")
	}
	return fn()
}

// RuntimeShutdownCall records one Runtime.Shutdown invocation.
type RuntimeShutdownCall struct {
	Context context.Context
}

func (d *Runtime) Shutdown(ctx context.Context) error {
	d.ShutdownCalls = append(d.ShutdownCalls, RuntimeShutdownCall{Context: ctx})
	fn := d.ShutdownFunc
	if fn == nil {
		panic("unexpected Runtime.Shutdown call")
	}
	return fn(ctx)
}
