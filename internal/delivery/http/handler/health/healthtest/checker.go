package healthtest

import (
	"context"

	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// Checker is a configurable double for its parent package's Checker contract.
type Checker struct {
	CheckFunc  func(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error)
	CheckCalls []CheckerCheckCall
}

// CheckerCheckCall records one Checker.Check invocation.
type CheckerCheckCall struct {
	Context context.Context
	Mode    usecasehealth.CheckMode
}

func (d *Checker) Check(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
	d.CheckCalls = append(d.CheckCalls, CheckerCheckCall{Context: ctx, Mode: mode})
	fn := d.CheckFunc
	if fn == nil {
		panic("unexpected Checker.Check call")
	}
	return fn(ctx, mode)
}
