package sessiontest

import (
	"context"
	"time"

	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
)

// SessionRepository is a configurable double for its parent package's SessionRepository contract.
type SessionRepository struct {
	CreateFunc      func(ctx context.Context, session authsession.SessionData, ttl time.Duration) error
	CreateCalls     []SessionRepositoryCreateCall
	GetByTokenFunc  func(ctx context.Context, token string) (authsession.SessionData, error)
	GetByTokenCalls []SessionRepositoryGetByTokenCall
	DeleteFunc      func(ctx context.Context, token string) error
	DeleteCalls     []SessionRepositoryDeleteCall
}

// SessionRepositoryCreateCall records one SessionRepository.Create invocation.
type SessionRepositoryCreateCall struct {
	Context context.Context
	Session authsession.SessionData
	TTL     time.Duration
}

func (d *SessionRepository) Create(ctx context.Context, session authsession.SessionData, ttl time.Duration) error {
	d.CreateCalls = append(d.CreateCalls, SessionRepositoryCreateCall{Context: ctx, Session: session, TTL: ttl})
	fn := d.CreateFunc
	if fn == nil {
		panic("unexpected SessionRepository.Create call")
	}
	return fn(ctx, session, ttl)
}

// SessionRepositoryGetByTokenCall records one SessionRepository.GetByToken invocation.
type SessionRepositoryGetByTokenCall struct {
	Context context.Context
	Token   string
}

func (d *SessionRepository) GetByToken(ctx context.Context, token string) (authsession.SessionData, error) {
	d.GetByTokenCalls = append(d.GetByTokenCalls, SessionRepositoryGetByTokenCall{Context: ctx, Token: token})
	fn := d.GetByTokenFunc
	if fn == nil {
		panic("unexpected SessionRepository.GetByToken call")
	}
	return fn(ctx, token)
}

// SessionRepositoryDeleteCall records one SessionRepository.Delete invocation.
type SessionRepositoryDeleteCall struct {
	Context context.Context
	Token   string
}

func (d *SessionRepository) Delete(ctx context.Context, token string) error {
	d.DeleteCalls = append(d.DeleteCalls, SessionRepositoryDeleteCall{Context: ctx, Token: token})
	fn := d.DeleteFunc
	if fn == nil {
		panic("unexpected SessionRepository.Delete call")
	}
	return fn(ctx, token)
}
