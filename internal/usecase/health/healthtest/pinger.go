package healthtest

import (
	"context"
)

// Pinger is a configurable double for its parent package's Pinger contract.
type Pinger struct {
	PingFunc  func(ctx context.Context) error
	PingCalls []PingerPingCall
}

// PingerPingCall records one Pinger.Ping invocation.
type PingerPingCall struct {
	Context context.Context
}

func (d *Pinger) Ping(ctx context.Context) error {
	d.PingCalls = append(d.PingCalls, PingerPingCall{Context: ctx})
	fn := d.PingFunc
	if fn == nil {
		panic("unexpected Pinger.Ping call")
	}
	return fn(ctx)
}
