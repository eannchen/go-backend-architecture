package observabilitytest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

// Runtime is the canonical configurable double for observability.Runtime.
type Runtime struct {
	LogEmitterFunc  func() observability.LogEmitter
	LogEmitterCalls int
	TracerFunc      func() observability.Tracer
	TracerCalls     int
	MeterFunc       func() observability.Meter
	MeterCalls      int
	ShutdownFunc    func(context.Context) error
	ShutdownCalls   int
}

func (r *Runtime) LogEmitter() observability.LogEmitter {
	r.LogEmitterCalls++
	if r.LogEmitterFunc == nil {
		panic("unexpected Runtime.LogEmitter call")
	}
	return r.LogEmitterFunc()
}

func (r *Runtime) Tracer() observability.Tracer {
	r.TracerCalls++
	if r.TracerFunc == nil {
		panic("unexpected Runtime.Tracer call")
	}
	return r.TracerFunc()
}

func (r *Runtime) Meter() observability.Meter {
	r.MeterCalls++
	if r.MeterFunc == nil {
		panic("unexpected Runtime.Meter call")
	}
	return r.MeterFunc()
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	r.ShutdownCalls++
	if r.ShutdownFunc == nil {
		panic("unexpected Runtime.Shutdown call")
	}
	return r.ShutdownFunc(ctx)
}

var _ observability.Runtime = (*Runtime)(nil)
