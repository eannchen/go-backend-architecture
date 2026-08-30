package sessiontest

import (
	"context"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
)

// SessionManager is a reusable test double for authsession.SessionManager.
type SessionManager struct {
	CreateFunc     func(context.Context, auth.Identity) (auth.Session, error)
	CreateCalls    int
	CreateIdentity auth.Identity
	ValidateFunc   func(context.Context, string) (auth.Session, error)
	ValidateCalls  int
	ValidateToken  string
	RevokeFunc     func(context.Context, string) error
	RevokeCalls    int
	RevokeToken    string
}

func (m *SessionManager) Create(ctx context.Context, identity auth.Identity) (auth.Session, error) {
	m.CreateCalls++
	m.CreateIdentity = identity
	if m.CreateFunc == nil {
		panic("unexpected SessionManager.Create call")
	}
	return m.CreateFunc(ctx, identity)
}

func (m *SessionManager) Validate(ctx context.Context, token string) (auth.Session, error) {
	m.ValidateCalls++
	m.ValidateToken = token
	if m.ValidateFunc == nil {
		panic("unexpected SessionManager.Validate call")
	}
	return m.ValidateFunc(ctx, token)
}

func (m *SessionManager) Revoke(ctx context.Context, token string) error {
	m.RevokeCalls++
	m.RevokeToken = token
	if m.RevokeFunc == nil {
		panic("unexpected SessionManager.Revoke call")
	}
	return m.RevokeFunc(ctx, token)
}

var _ authsession.SessionManager = (*SessionManager)(nil)
