package session

import (
	"context"
	"errors"
	"time"
)

// ErrSessionNotFound means the token has no active session.
var ErrSessionNotFound = errors.New("session not found")

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
	// GetByToken returns ErrSessionNotFound when the token has no stored record.
	GetByToken(ctx context.Context, token string) (SessionData, error)
	Delete(ctx context.Context, token string) error
}
