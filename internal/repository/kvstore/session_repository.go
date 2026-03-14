package kvstore

import (
	"context"
	"time"
)

// SessionData is the stored representation of an active session.
type SessionData struct {
	Token     string
	UserID    int64
	Email     string
	Method    string
	ExpiresAt time.Time
}

// SessionRepository persists session state.
type SessionRepository interface {
	Create(ctx context.Context, session SessionData, ttl time.Duration) error
	GetByToken(ctx context.Context, token string) (SessionData, error)
	Delete(ctx context.Context, token string) error
}
