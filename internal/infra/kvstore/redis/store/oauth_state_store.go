package store

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

const oauthStateKeyPrefix = "oauth_state:"

// OAuthStateStore implements OAuthStateRepository using Redis with TTL for automatic expiry.
type OAuthStateStore struct {
	client *goredis.Client
}

func NewOAuthStateStore(client *goredis.Client) *OAuthStateStore {
	return &OAuthStateStore{client: client}
}

func (s *OAuthStateStore) Store(ctx context.Context, state string, ttl time.Duration) error {
	if err := s.client.Set(ctx, oauthStateKeyPrefix+state, "1", ttl).Err(); err != nil {
		return fmt.Errorf("store oauth state: %w", err)
	}
	return nil
}

// Consume atomically deletes the state and returns whether it existed.
func (s *OAuthStateStore) Consume(ctx context.Context, state string) (bool, error) {
	result, err := s.client.Del(ctx, oauthStateKeyPrefix+state).Result()
	if err != nil {
		return false, fmt.Errorf("consume oauth state: %w", err)
	}
	return result > 0, nil
}

var _ repokvstore.OAuthStateRepository = (*OAuthStateStore)(nil)
