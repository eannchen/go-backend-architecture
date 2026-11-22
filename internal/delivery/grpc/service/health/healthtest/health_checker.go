package healthtest

import (
	"context"

	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// HealthChecker is a configurable double for its parent package's HealthChecker contract.
type HealthChecker struct {
	CheckFunc  func(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error)
	CheckCalls []HealthCheckerCheckCall
}

// HealthCheckerCheckCall records one HealthChecker.Check invocation.
type HealthCheckerCheckCall struct {
	Context context.Context
	Mode    usecasehealth.CheckMode
}

func (d *HealthChecker) Check(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
	d.CheckCalls = append(d.CheckCalls, HealthCheckerCheckCall{Context: ctx, Mode: mode})
	fn := d.CheckFunc
	if fn == nil {
		panic("unexpected HealthChecker.Check call")
	}
	return fn(ctx, mode)
}
