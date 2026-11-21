package authtest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// SessionManager is a configurable double for its parent package's SessionManager contract.
type SessionManager struct {
	CreateFunc  func(ctx context.Context, identity auth.Identity) (auth.Session, error)
	CreateCalls []SessionManagerCreateCall
	RevokeFunc  func(ctx context.Context, token string) error
	RevokeCalls []SessionManagerRevokeCall
}

// SessionManagerCreateCall records one SessionManager.Create invocation.
type SessionManagerCreateCall struct {
	Context  context.Context
	Identity auth.Identity
}

func (d *SessionManager) Create(ctx context.Context, identity auth.Identity) (auth.Session, error) {
	d.CreateCalls = append(d.CreateCalls, SessionManagerCreateCall{Context: ctx, Identity: identity})
	fn := d.CreateFunc
	if fn == nil {
		panic("unexpected SessionManager.Create call")
	}
	return fn(ctx, identity)
}

// SessionManagerRevokeCall records one SessionManager.Revoke invocation.
type SessionManagerRevokeCall struct {
	Context context.Context
	Token   string
}

func (d *SessionManager) Revoke(ctx context.Context, token string) error {
	d.RevokeCalls = append(d.RevokeCalls, SessionManagerRevokeCall{Context: ctx, Token: token})
	fn := d.RevokeFunc
	if fn == nil {
		panic("unexpected SessionManager.Revoke call")
	}
	return fn(ctx, token)
}
