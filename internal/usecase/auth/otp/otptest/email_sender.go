package otptest

import (
	"context"
)

// EmailSender is a configurable double for its parent package's EmailSender contract.
type EmailSender struct {
	SendOTPFunc  func(ctx context.Context, email, code string) error
	SendOTPCalls []EmailSenderSendOTPCall
}

// EmailSenderSendOTPCall records one EmailSender.SendOTP invocation.
type EmailSenderSendOTPCall struct {
	Context context.Context
	Email   string
	Code    string
}

func (d *EmailSender) SendOTP(ctx context.Context, email, code string) error {
	d.SendOTPCalls = append(d.SendOTPCalls, EmailSenderSendOTPCall{Context: ctx, Email: email, Code: code})
	fn := d.SendOTPFunc
	if fn == nil {
		panic("unexpected EmailSender.SendOTP call")
	}
	return fn(ctx, email, code)
}
