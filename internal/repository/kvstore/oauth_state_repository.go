package kvstore

import (
	"context"
	"time"
)

// OAuthStateData is the server-held binding for one OAuth authorization flow.
// BrowserBindingHash is a SHA-256 digest of the HttpOnly cookie value; keeping
// only the digest means a Redis read cannot be used to complete the callback.
type OAuthStateData struct {
	BrowserBindingHash string
}

// OAuthStateRepository manages one-time, browser-bound CSRF state for OAuth flows.
type OAuthStateRepository interface {
	Store(ctx context.Context, state string, data OAuthStateData, ttl time.Duration) error
	// Consume atomically reads and deletes the state token. found is false when
	// the token is missing or expired.
	Consume(ctx context.Context, state string) (data OAuthStateData, found bool, err error)
}
