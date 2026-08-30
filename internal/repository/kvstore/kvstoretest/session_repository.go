package kvstoretest

import (
	"context"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

// SessionRepository is a reusable test double for repokvstore.SessionRepository.
type SessionRepository struct {
	CreateFunc      func(context.Context, repokvstore.SessionData, time.Duration) error
	CreateCalls     int
	CreateSession   repokvstore.SessionData
	CreateTTL       time.Duration
	GetByTokenFunc  func(context.Context, string) (repokvstore.SessionData, error)
	GetByTokenCalls int
	GetByTokenToken string
	DeleteFunc      func(context.Context, string) error
	DeleteCalls     int
	DeleteToken     string
}

func (r *SessionRepository) Create(ctx context.Context, session repokvstore.SessionData, ttl time.Duration) error {
	r.CreateCalls++
	r.CreateSession = session
	r.CreateTTL = ttl
	if r.CreateFunc == nil {
		panic("unexpected SessionRepository.Create call")
	}
	return r.CreateFunc(ctx, session, ttl)
}

func (r *SessionRepository) GetByToken(ctx context.Context, token string) (repokvstore.SessionData, error) {
	r.GetByTokenCalls++
	r.GetByTokenToken = token
	if r.GetByTokenFunc == nil {
		panic("unexpected SessionRepository.GetByToken call")
	}
	return r.GetByTokenFunc(ctx, token)
}

func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	r.DeleteCalls++
	r.DeleteToken = token
	if r.DeleteFunc == nil {
		panic("unexpected SessionRepository.Delete call")
	}
	return r.DeleteFunc(ctx, token)
}

var _ repokvstore.SessionRepository = (*SessionRepository)(nil)
