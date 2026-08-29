package otptest

import (
	"context"

	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/otp"
)

// EmailSender is a reusable test double for repoexternal.EmailSender.
type EmailSender struct {
	SendOTPFunc  func(context.Context, string, string) error
	SendOTPCalls int
	SendOTPEmail string
	SendOTPCode  string
}

func (s *EmailSender) SendOTP(ctx context.Context, email, code string) error {
	s.SendOTPCalls++
	s.SendOTPEmail = email
	s.SendOTPCode = code
	if s.SendOTPFunc == nil {
		panic("unexpected EmailSender.SendOTP call")
	}
	return s.SendOTPFunc(ctx, email, code)
}

var _ repoexternal.EmailSender = (*EmailSender)(nil)
