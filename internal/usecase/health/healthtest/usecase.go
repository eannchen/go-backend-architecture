package healthtest

import (
	"context"

	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// Usecase is the canonical configurable double for usecasehealth.Usecase.
type Usecase struct {
	CheckFunc  func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error)
	CheckCalls int
	CheckMode  usecasehealth.CheckMode
}

func (u *Usecase) Check(ctx context.Context, mode usecasehealth.CheckMode) (usecasehealth.Result, error) {
	u.CheckCalls++
	u.CheckMode = mode
	if u.CheckFunc == nil {
		panic("unexpected health Usecase.Check call")
	}
	return u.CheckFunc(ctx, mode)
}

var _ usecasehealth.Usecase = (*Usecase)(nil)
