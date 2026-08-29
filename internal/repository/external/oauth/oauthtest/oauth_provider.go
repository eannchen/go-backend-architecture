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
	if p.AuthCodeURLFunc == nil {
		panic("unexpected OAuthProvider.AuthCodeURL call")
	}
	return p.AuthCodeURLFunc(state, codeVerifier)
}

func (p *OAuthProvider) Exchange(ctx context.Context, code, codeVerifier string) (repoexternal.OAuthUserInfo, error) {
	p.ExchangeCalls++
	p.ExchangeCode = code
	p.ExchangeVerifier = codeVerifier
	if p.ExchangeFunc == nil {
		panic("unexpected OAuthProvider.Exchange call")
	}
	return p.ExchangeFunc(ctx, code, codeVerifier)
}

var _ repoexternal.OAuthProvider = (*OAuthProvider)(nil)
