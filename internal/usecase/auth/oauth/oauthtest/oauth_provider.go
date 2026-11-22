package oauthtest

import (
	"context"

	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// OAuthProvider is a configurable double for its parent package's OAuthProvider contract.
type OAuthProvider struct {
	NameFunc         func() string
	NameCalls        []OAuthProviderNameCall
	AuthCodeURLFunc  func(state, codeVerifier string) string
	AuthCodeURLCalls []OAuthProviderAuthCodeURLCall
	ExchangeFunc     func(ctx context.Context, code, codeVerifier string) (authoauth.OAuthUserInfo, error)
	ExchangeCalls    []OAuthProviderExchangeCall
}

// OAuthProviderNameCall records one OAuthProvider.Name invocation.
type OAuthProviderNameCall struct {
}

func (d *OAuthProvider) Name() string {
	d.NameCalls = append(d.NameCalls, OAuthProviderNameCall{})
	fn := d.NameFunc
	if fn == nil {
		panic("unexpected OAuthProvider.Name call")
	}
	return fn()
}

// OAuthProviderAuthCodeURLCall records one OAuthProvider.AuthCodeURL invocation.
type OAuthProviderAuthCodeURLCall struct {
	State        string
	CodeVerifier string
}

func (d *OAuthProvider) AuthCodeURL(state, codeVerifier string) string {
	d.AuthCodeURLCalls = append(d.AuthCodeURLCalls, OAuthProviderAuthCodeURLCall{State: state, CodeVerifier: codeVerifier})
	fn := d.AuthCodeURLFunc
	if fn == nil {
		panic("unexpected OAuthProvider.AuthCodeURL call")
	}
	return fn(state, codeVerifier)
}

// OAuthProviderExchangeCall records one OAuthProvider.Exchange invocation.
type OAuthProviderExchangeCall struct {
	Context      context.Context
	Code         string
	CodeVerifier string
}

func (d *OAuthProvider) Exchange(ctx context.Context, code, codeVerifier string) (authoauth.OAuthUserInfo, error) {
	d.ExchangeCalls = append(d.ExchangeCalls, OAuthProviderExchangeCall{Context: ctx, Code: code, CodeVerifier: codeVerifier})
	fn := d.ExchangeFunc
	if fn == nil {
		panic("unexpected OAuthProvider.Exchange call")
	}
	return fn(ctx, code, codeVerifier)
}
