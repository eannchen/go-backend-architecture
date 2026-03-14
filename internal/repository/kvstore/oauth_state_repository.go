package kvstore

import (
	"context"
	"time"
)

// OAuthStateRepository manages CSRF state tokens for OAuth flows.
type OAuthStateRepository interface {
	Store(ctx context.Context, state string, ttl time.Duration) error
	// Consume atomically checks and deletes the state token; returns true if it existed.
	Consume(ctx context.Context, state string) (bool, error)
}
