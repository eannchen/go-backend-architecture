package oauthtest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// OAuthAuthenticator is a reusable test double for authoauth.OAuthAuthenticator.
type OAuthAuthenticator struct {
	AuthorizeURLFunc       func(context.Context, string) (string, error)
	AuthorizeURLCalls      int
	AuthorizeURLProvider   string
	HandleCallbackFunc     func(context.Context, string, string, string) (auth.Identity, error)
	HandleCallbackCalls    int
	HandleCallbackProvider string
	HandleCallbackCode     string
	HandleCallbackState    string
}

func (a *OAuthAuthenticator) AuthorizeURL(ctx context.Context, provider string) (string, error) {
	a.AuthorizeURLCalls++
	a.AuthorizeURLProvider = provider
	if a.AuthorizeURLFunc != nil {
		return a.AuthorizeURLFunc(ctx, provider)
	}
	return "", nil
}

func (a *OAuthAuthenticator) HandleCallback(ctx context.Context, provider, code, state string) (auth.Identity, error) {
	a.HandleCallbackCalls++
	a.HandleCallbackProvider = provider
	a.HandleCallbackCode = code
	a.HandleCallbackState = state
	if a.HandleCallbackFunc != nil {
		return a.HandleCallbackFunc(ctx, provider, code, state)
	}
	return auth.Identity{}, nil
}

var _ authoauth.OAuthAuthenticator = (*OAuthAuthenticator)(nil)
