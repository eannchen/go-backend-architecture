package stub

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// StubSender logs OTP codes instead of sending real emails.
type StubSender struct {
	log logger.Logger
}

func NewStubSender(log logger.Logger) *StubSender {
	return &StubSender{log: log}
}

func (s *StubSender) SendOTP(ctx context.Context, email, code string) error {
	s.log.Info(ctx, "stub email: OTP code generated",
		logger.FromPairs("email", email, "otp_code", code),
	)
	return nil
}
