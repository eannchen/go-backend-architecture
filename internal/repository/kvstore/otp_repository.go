package kvstore

import (
	"context"
	"time"
)

// OTPRepository manages one-time-password storage with automatic expiry.
type OTPRepository interface {
	Store(ctx context.Context, email, hashedCode string, ttl time.Duration) error
	Get(ctx context.Context, email string) (hashedCode string, err error)
	Delete(ctx context.Context, email string) error
}
