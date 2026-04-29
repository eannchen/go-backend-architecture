package oauthtest

import (
	"context"

	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/oauth"
)

// OAuthProvider is a reusable test double for repoexternal.OAuthProvider.
type OAuthProvider struct {
	ProviderName    string
	AuthCodeURLFunc func(string) string
	AuthCodeCalls   int
	AuthCodeState   string
	ExchangeFunc    func(context.Context, string) (repoexternal.OAuthUserInfo, error)
	ExchangeCalls   int
	ExchangeCode    string
}

func (p *OAuthProvider) Name() string {
	if p.ProviderName == "" {
		return "test"
	}
	return p.ProviderName
}

func (p *OAuthProvider) AuthCodeURL(state string) string {
	p.AuthCodeCalls++
	p.AuthCodeState = state
	if p.AuthCodeURLFunc != nil {
		return p.AuthCodeURLFunc(state)
	}
	return "https://oauth.test/authorize?state=" + state
}

func (p *OAuthProvider) Exchange(ctx context.Context, code string) (repoexternal.OAuthUserInfo, error) {
	p.ExchangeCalls++
	p.ExchangeCode = code
	if p.ExchangeFunc != nil {
		return p.ExchangeFunc(ctx, code)
	}
	return repoexternal.OAuthUserInfo{}, nil
}

var _ repoexternal.OAuthProvider = (*OAuthProvider)(nil)
