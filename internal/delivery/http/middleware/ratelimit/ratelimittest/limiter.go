package ratelimittest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
)

// Limiter is a configurable double for its parent package's Limiter contract.
type Limiter struct {
	AllowIPFunc  func(ctx context.Context, ip string) (globalratelimit.Decision, error)
	AllowIPCalls []LimiterAllowIPCall
}

// LimiterAllowIPCall records one Limiter.AllowIP invocation.
type LimiterAllowIPCall struct {
	Context context.Context
	IP      string
}

func (d *Limiter) AllowIP(ctx context.Context, ip string) (globalratelimit.Decision, error) {
	d.AllowIPCalls = append(d.AllowIPCalls, LimiterAllowIPCall{Context: ctx, IP: ip})
	fn := d.AllowIPFunc
	if fn == nil {
		panic("unexpected Limiter.AllowIP call")
	}
	return fn(ctx, ip)
}
