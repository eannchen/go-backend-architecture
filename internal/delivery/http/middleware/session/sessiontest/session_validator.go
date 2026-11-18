package sessiontest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// SessionValidator is a configurable double for its parent package's SessionValidator contract.
type SessionValidator struct {
	ValidateFunc  func(ctx context.Context, token string) (auth.Session, error)
	ValidateCalls []SessionValidatorValidateCall
}

// SessionValidatorValidateCall records one SessionValidator.Validate invocation.
type SessionValidatorValidateCall struct {
	Context context.Context
	Token   string
}

func (d *SessionValidator) Validate(ctx context.Context, token string) (auth.Session, error) {
	d.ValidateCalls = append(d.ValidateCalls, SessionValidatorValidateCall{Context: ctx, Token: token})
	fn := d.ValidateFunc
	if fn == nil {
		panic("unexpected SessionValidator.Validate call")
	}
	return fn(ctx, token)
}
