package store

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

const otpKeyPrefix = "otp:"

var consumeOTPScript = goredis.NewScript(`
local stored = redis.call('GET', KEYS[1])

if not stored then
    return 0
end

if stored ~= ARGV[1] then
    return 1
end

redis.call('DEL', KEYS[1])
return 2`)

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

func (s *OTPStore) Consume(ctx context.Context, email, candidateHash string) (bool, error) {
	result, err := consumeOTPScript.Run(ctx, s.client, []string{otpKeyPrefix + email}, candidateHash).Result()
	if err != nil {
		return false, fmt.Errorf("consume otp: %w", err)
	}

	status, err := redisInt(result)
	if err != nil {
		return false, fmt.Errorf("consume otp: %w", err)
	}
	switch status {
	case 0:
		return false, repokvstore.ErrOTPNotFound
	case 1:
		return false, nil
	case 2:
		return true, nil
	default:
		return false, fmt.Errorf("consume otp: unexpected script status %d", status)
	}
}

func (s *OTPStore) Delete(ctx context.Context, email string) error {
	if err := s.client.Del(ctx, otpKeyPrefix+email).Err(); err != nil {
		return fmt.Errorf("delete otp: %w", err)
	}
	return nil
}

var _ repokvstore.OTPRepository = (*OTPStore)(nil)
