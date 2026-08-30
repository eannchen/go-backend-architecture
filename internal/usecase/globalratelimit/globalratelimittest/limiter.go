package globalratelimittest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
)

// Limiter is the canonical configurable double for globalratelimit.Limiter.
type Limiter struct {
	AllowIPFunc  func(context.Context, string) (globalratelimit.Decision, error)
	AllowIPCalls int
	AllowIPIP    string
}

func (l *Limiter) AllowIP(ctx context.Context, ip string) (globalratelimit.Decision, error) {
	l.AllowIPCalls++
	l.AllowIPIP = ip
	if l.AllowIPFunc == nil {
		panic("unexpected Limiter.AllowIP call")
	}
	return l.AllowIPFunc(ctx, ip)
}

var _ globalratelimit.Limiter = (*Limiter)(nil)
