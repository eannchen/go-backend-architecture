package otptest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
)

// OTPAuthenticator is a reusable test double for authotp.OTPAuthenticator.
type OTPAuthenticator struct {
	SendCodeFunc    func(context.Context, string) error
	SendCodeCalls   int
	SendCodeEmail   string
	VerifyCodeFunc  func(context.Context, string, string) (auth.Identity, error)
	VerifyCodeCalls int
	VerifyCodeEmail string
	VerifyCodeCode  string
}

func (a *OTPAuthenticator) SendCode(ctx context.Context, email string) error {
	a.SendCodeCalls++
	a.SendCodeEmail = email
	if a.SendCodeFunc == nil {
		panic("unexpected OTPAuthenticator.SendCode call")
	}
	return a.SendCodeFunc(ctx, email)
}

func (a *OTPAuthenticator) VerifyCode(ctx context.Context, email, code string) (auth.Identity, error) {
	a.VerifyCodeCalls++
	a.VerifyCodeEmail = email
	a.VerifyCodeCode = code
	if a.VerifyCodeFunc == nil {
		panic("unexpected OTPAuthenticator.VerifyCode call")
	}
	return a.VerifyCodeFunc(ctx, email, code)
}

var _ authotp.OTPAuthenticator = (*OTPAuthenticator)(nil)
