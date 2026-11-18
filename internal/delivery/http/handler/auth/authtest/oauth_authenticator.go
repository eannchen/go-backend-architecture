package authtest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// OAuthAuthenticator is a configurable double for its parent package's OAuthAuthenticator contract.
type OAuthAuthenticator struct {
	AuthorizeFunc       func(ctx context.Context, provider string) (authoauth.Authorization, error)
	AuthorizeCalls      []OAuthAuthenticatorAuthorizeCall
	HandleCallbackFunc  func(ctx context.Context, provider, code, state, browserBinding string) (auth.Identity, error)
	HandleCallbackCalls []OAuthAuthenticatorHandleCallbackCall
}

// OAuthAuthenticatorAuthorizeCall records one OAuthAuthenticator.Authorize invocation.
type OAuthAuthenticatorAuthorizeCall struct {
	Context  context.Context
	Provider string
}

func (d *OAuthAuthenticator) Authorize(ctx context.Context, provider string) (authoauth.Authorization, error) {
	d.AuthorizeCalls = append(d.AuthorizeCalls, OAuthAuthenticatorAuthorizeCall{Context: ctx, Provider: provider})
	fn := d.AuthorizeFunc
	if fn == nil {
		panic("unexpected OAuthAuthenticator.Authorize call")
	}
	return fn(ctx, provider)
}

// OAuthAuthenticatorHandleCallbackCall records one OAuthAuthenticator.HandleCallback invocation.
type OAuthAuthenticatorHandleCallbackCall struct {
	Context        context.Context
	Provider       string
	Code           string
	State          string
	BrowserBinding string
}

func (d *OAuthAuthenticator) HandleCallback(ctx context.Context, provider, code, state, browserBinding string) (auth.Identity, error) {
	d.HandleCallbackCalls = append(d.HandleCallbackCalls, OAuthAuthenticatorHandleCallbackCall{Context: ctx, Provider: provider, Code: code, State: state, BrowserBinding: browserBinding})
	fn := d.HandleCallbackFunc
	if fn == nil {
		panic("unexpected OAuthAuthenticator.HandleCallback call")
	}
	return fn(ctx, provider, code, state, browserBinding)
}
