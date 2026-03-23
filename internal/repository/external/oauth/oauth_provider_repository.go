package external

import "context"

// OAuthUserInfo holds user profile data returned by an OAuth provider after token exchange.
type OAuthUserInfo struct {
	ProviderUserID string
	Email          string
}

// OAuthProvider interacts with a single OAuth2 provider (e.g. Google, GitHub).
type OAuthProvider interface {
	Name() string
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (OAuthUserInfo, error)
}
