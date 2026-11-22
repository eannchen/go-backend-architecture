package authtest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// OTPAuthenticator is a configurable double for its parent package's OTPAuthenticator contract.
type OTPAuthenticator struct {
	SendCodeFunc    func(ctx context.Context, email string) error
	SendCodeCalls   []OTPAuthenticatorSendCodeCall
	VerifyCodeFunc  func(ctx context.Context, email, code string) (auth.Identity, error)
	VerifyCodeCalls []OTPAuthenticatorVerifyCodeCall
}

// OTPAuthenticatorSendCodeCall records one OTPAuthenticator.SendCode invocation.
type OTPAuthenticatorSendCodeCall struct {
	Context context.Context
	Email   string
}

func (d *OTPAuthenticator) SendCode(ctx context.Context, email string) error {
	d.SendCodeCalls = append(d.SendCodeCalls, OTPAuthenticatorSendCodeCall{Context: ctx, Email: email})
	fn := d.SendCodeFunc
	if fn == nil {
		panic("unexpected OTPAuthenticator.SendCode call")
	}
	return fn(ctx, email)
}

// OTPAuthenticatorVerifyCodeCall records one OTPAuthenticator.VerifyCode invocation.
type OTPAuthenticatorVerifyCodeCall struct {
	Context context.Context
	Email   string
	Code    string
}

func (d *OTPAuthenticator) VerifyCode(ctx context.Context, email, code string) (auth.Identity, error) {
	d.VerifyCodeCalls = append(d.VerifyCodeCalls, OTPAuthenticatorVerifyCodeCall{Context: ctx, Email: email, Code: code})
	fn := d.VerifyCodeFunc
	if fn == nil {
		panic("unexpected OTPAuthenticator.VerifyCode call")
	}
	return fn(ctx, email, code)
}
