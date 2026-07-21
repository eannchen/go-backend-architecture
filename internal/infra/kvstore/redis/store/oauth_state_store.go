package store

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

const oauthStateKeyPrefix = "oauth_state:"

// consumeOAuthStateScript reads and deletes in one Redis operation so a state
// cannot be used by two concurrent callbacks.
var consumeOAuthStateScript = goredis.NewScript(`
local value = redis.call('GET', KEYS[1])
if value == false then
    return false
end
redis.call('DEL', KEYS[1])
return value
`)

// OAuthStateStore implements OAuthStateRepository using Redis with TTL for automatic expiry.
type OAuthStateStore struct {
	client *goredis.Client
}

func NewOAuthStateStore(client *goredis.Client) *OAuthStateStore {
	return &OAuthStateStore{client: client}
}

func (s *OAuthStateStore) Store(ctx context.Context, state string, data repokvstore.OAuthStateData, ttl time.Duration) error {
	if err := s.client.Set(ctx, oauthStateKeyPrefix+state, data.BrowserBindingHash, ttl).Err(); err != nil {
		return fmt.Errorf("store oauth state: %w", err)
	}
	return nil
}

// Consume atomically reads and deletes the state binding.
func (s *OAuthStateStore) Consume(ctx context.Context, state string) (repokvstore.OAuthStateData, bool, error) {
	value, err := consumeOAuthStateScript.Run(ctx, s.client, []string{oauthStateKeyPrefix + state}).Text()
	if err != nil {
		if err == goredis.Nil {
			return repokvstore.OAuthStateData{}, false, nil
		}
		return repokvstore.OAuthStateData{}, false, fmt.Errorf("consume oauth state: %w", err)
	}
	return repokvstore.OAuthStateData{BrowserBindingHash: value}, true, nil
}

var _ repokvstore.OAuthStateRepository = (*OAuthStateStore)(nil)
