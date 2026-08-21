package healthtest

import (
	"context"

	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// Usecase is a reusable test double for health.Usecase.
type Usecase struct {
	CheckFunc  func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error)
	CheckCalls int
	CheckModes []usecasehealth.CheckMode
}

func (u *Usecase) Check(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
	u.CheckCalls++
	u.CheckModes = append(u.CheckModes, mode)
	if u.CheckFunc != nil {
		return u.CheckFunc(ctx, mode)
	}
	return usecasehealth.Result{}, nil
}

var _ usecasehealth.Usecase = (*Usecase)(nil)
