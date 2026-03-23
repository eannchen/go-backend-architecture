package external

import "context"

// EmailSender sends transactional emails (e.g. OTP codes, notifications).
type EmailSender interface {
	SendOTP(ctx context.Context, email, code string) error
}
