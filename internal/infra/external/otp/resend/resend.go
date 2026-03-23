package resend

import (
	"context"
	"fmt"
	"html"

	"github.com/resend/resend-go/v3"
	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/otp"
)

// ResendSender sends OTP emails via Resend API.
type ResendSender struct {
	client *resend.Client
	from   string
}

// NewResendSender builds an EmailSender that uses Resend. apiKey and from must be non-empty.
func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

// SendOTP sends the OTP email via Resend. code is assumed server-generated numeric only; escaped for HTML safety.
func (s *ResendSender) SendOTP(ctx context.Context, email, code string) error {
	_, err := s.client.Emails.Send(&resend.SendEmailRequest{
		From:    s.from,
		To:      []string{email},
		Subject: "Your verification code",
		Html:    "<p>Your verification code is: <strong>" + html.EscapeString(code) + "</strong></p>",
	})
	if err != nil {
		return fmt.Errorf("send otp: %w", err)
	}
	return nil
}

var _ repoexternal.EmailSender = (*ResendSender)(nil)
