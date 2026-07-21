package oauthtest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// OAuthAuthenticator is a reusable test double for authoauth.OAuthAuthenticator.
type OAuthAuthenticator struct {
	AuthorizeFunc          func(context.Context, string) (authoauth.Authorization, error)
	AuthorizeCalls         int
	AuthorizeProvider      string
	HandleCallbackFunc     func(context.Context, string, string, string, string) (auth.Identity, error)
	HandleCallbackCalls    int
	HandleCallbackProvider string
	HandleCallbackCode     string
	HandleCallbackState    string
	HandleCallbackBinding  string
}

func (a *OAuthAuthenticator) Authorize(ctx context.Context, provider string) (authoauth.Authorization, error) {
	a.AuthorizeCalls++
	a.AuthorizeProvider = provider
	if a.AuthorizeFunc != nil {
		return a.AuthorizeFunc(ctx, provider)
	}
	return authoauth.Authorization{}, nil
}

func (a *OAuthAuthenticator) HandleCallback(ctx context.Context, provider, code, state, browserBinding string) (auth.Identity, error) {
	a.HandleCallbackCalls++
	a.HandleCallbackProvider = provider
	a.HandleCallbackCode = code
	a.HandleCallbackState = state
	a.HandleCallbackBinding = browserBinding
	if a.HandleCallbackFunc != nil {
		return a.HandleCallbackFunc(ctx, provider, code, state, browserBinding)
	}
	return auth.Identity{}, nil
}

var _ authoauth.OAuthAuthenticator = (*OAuthAuthenticator)(nil)
