package otptest

import (
	"context"

	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/otp"
)

// EmailSender is a reusable test double for repoexternal.EmailSender.
type EmailSender struct {
	SendOTPFunc  func(context.Context, string, string) error
	SendOTPCalls int
	Email        string
	Code         string
}

func (s *EmailSender) SendOTP(ctx context.Context, email, code string) error {
	s.SendOTPCalls++
	s.Email = email
	s.Code = code
	if s.SendOTPFunc != nil {
		return s.SendOTPFunc(ctx, email, code)
	}
	return nil
}

var _ repoexternal.EmailSender = (*EmailSender)(nil)
