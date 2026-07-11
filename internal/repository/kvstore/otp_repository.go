package kvstore

import (
	"context"
	"time"
)

// OTPRepository manages one-time-password storage with automatic expiry.
type OTPRepository interface {
	Store(ctx context.Context, email, hashedCode string, ttl time.Duration) error
	// Consume atomically compares the candidate hash and deletes the code only on a match.
	Consume(ctx context.Context, email, candidateHash string) (matched bool, err error)
	Delete(ctx context.Context, email string) error
}
