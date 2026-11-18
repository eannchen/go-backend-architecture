package oauth

import (
	"context"
	"time"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
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

// OAuthUserInfo holds user profile data returned by an OAuth provider after token exchange.
type OAuthUserInfo struct {
	ProviderUserID string
	Email          string
}

// OAuthProvider interacts with a single OAuth2 provider (e.g. Google, GitHub).
type OAuthProvider interface {
	Name() string
	AuthCodeURL(state, codeVerifier string) string
	Exchange(ctx context.Context, code, codeVerifier string) (OAuthUserInfo, error)
}

// OAuthUserUpsert identifies the user to find or create during an OAuth callback.
type OAuthUserUpsert struct {
	Provider       string
	ProviderUserID string
	Email          string
}

// UserRepository performs the atomic user update required by the callback.
type UserRepository interface {
	UpsertOAuthUser(context.Context, OAuthUserUpsert) (domainuser.User, error)
}
