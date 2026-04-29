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
	Created         repokvstore.SessionData
	CreatedTTL      time.Duration
	GetByTokenFunc  func(context.Context, string) (repokvstore.SessionData, error)
	GetByTokenCalls int
	GetByTokenToken string
	DeleteFunc      func(context.Context, string) error
	DeleteCalls     int
	DeleteToken     string
}

func (r *SessionRepository) Create(ctx context.Context, session repokvstore.SessionData, ttl time.Duration) error {
	r.CreateCalls++
	r.Created = session
	r.CreatedTTL = ttl
	if r.CreateFunc != nil {
		return r.CreateFunc(ctx, session, ttl)
	}
	return nil
}

func (r *SessionRepository) GetByToken(ctx context.Context, token string) (repokvstore.SessionData, error) {
	r.GetByTokenCalls++
	r.GetByTokenToken = token
	if r.GetByTokenFunc != nil {
		return r.GetByTokenFunc(ctx, token)
	}
	return repokvstore.SessionData{}, nil
}

func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	r.DeleteCalls++
	r.DeleteToken = token
	if r.DeleteFunc != nil {
		return r.DeleteFunc(ctx, token)
	}
	return nil
}

var _ repokvstore.SessionRepository = (*SessionRepository)(nil)
