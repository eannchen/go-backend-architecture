package store

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

const otpKeyPrefix = "otp:"

// OTPStore implements OTPRepository using Redis with automatic TTL expiry.
type OTPStore struct {
	client *goredis.Client
}

func NewOTPStore(client *goredis.Client) *OTPStore {
	return &OTPStore{client: client}
}

func (s *OTPStore) Store(ctx context.Context, email, hashedCode string, ttl time.Duration) error {
	if err := s.client.Set(ctx, otpKeyPrefix+email, hashedCode, ttl).Err(); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}
	return nil
}

func (s *OTPStore) Get(ctx context.Context, email string) (string, error) {
	code, err := s.client.Get(ctx, otpKeyPrefix+email).Result()
	if err != nil {
		return "", fmt.Errorf("get otp: %w", err)
	}
	return code, nil
}

func (s *OTPStore) Delete(ctx context.Context, email string) error {
	if err := s.client.Del(ctx, otpKeyPrefix+email).Err(); err != nil {
		return fmt.Errorf("delete otp: %w", err)
	}
	return nil
}

var _ repokvstore.OTPRepository = (*OTPStore)(nil)
