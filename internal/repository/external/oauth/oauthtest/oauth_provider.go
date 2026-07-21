package oauthtest

import (
	"context"

	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/oauth"
)

// OAuthProvider is a reusable test double for repoexternal.OAuthProvider.
type OAuthProvider struct {
	ProviderName     string
	AuthCodeURLFunc  func(string, string) string
	AuthCodeCalls    int
	AuthCodeState    string
	AuthCodeVerifier string
	ExchangeFunc     func(context.Context, string, string) (repoexternal.OAuthUserInfo, error)
	ExchangeCalls    int
	ExchangeCode     string
	ExchangeVerifier string
}

func (p *OAuthProvider) Name() string {
	if p.ProviderName == "" {
		return "test"
	}
	return p.ProviderName
}

func (p *OAuthProvider) AuthCodeURL(state, codeVerifier string) string {
	p.AuthCodeCalls++
	p.AuthCodeState = state
	p.AuthCodeVerifier = codeVerifier
	if p.AuthCodeURLFunc != nil {
		return p.AuthCodeURLFunc(state, codeVerifier)
	}
	return "https://oauth.test/authorize?state=" + state
}

func (p *OAuthProvider) Exchange(ctx context.Context, code, codeVerifier string) (repoexternal.OAuthUserInfo, error) {
	p.ExchangeCalls++
	p.ExchangeCode = code
	p.ExchangeVerifier = codeVerifier
	if p.ExchangeFunc != nil {
		return p.ExchangeFunc(ctx, code, codeVerifier)
	}
	return repoexternal.OAuthUserInfo{}, nil
}

var _ repoexternal.OAuthProvider = (*OAuthProvider)(nil)
