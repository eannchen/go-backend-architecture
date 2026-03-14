package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

const sessionKeyPrefix = "session:"

// SessionStore implements SessionRepository using Redis with JSON values and TTL.
type SessionStore struct {
	client *goredis.Client
}

func NewSessionStore(client *goredis.Client) *SessionStore {
	return &SessionStore{client: client}
}

type sessionJSON struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	Method    string `json:"method"`
	ExpiresAt int64  `json:"expires_at"`
}

func (s *SessionStore) Create(ctx context.Context, session repokvstore.SessionData, ttl time.Duration) error {
	data, err := json.Marshal(sessionJSON{
		UserID:    session.UserID,
		Email:     session.Email,
		Method:    session.Method,
		ExpiresAt: session.ExpiresAt.Unix(),
	})
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := s.client.Set(ctx, sessionKeyPrefix+session.Token, data, ttl).Err(); err != nil {
		return fmt.Errorf("store session: %w", err)
	}
	return nil
}

func (s *SessionStore) GetByToken(ctx context.Context, token string) (repokvstore.SessionData, error) {
	data, err := s.client.Get(ctx, sessionKeyPrefix+token).Bytes()
	if err != nil {
		return repokvstore.SessionData{}, fmt.Errorf("get session: %w", err)
	}
	var v sessionJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return repokvstore.SessionData{}, fmt.Errorf("unmarshal session: %w", err)
	}
	return repokvstore.SessionData{
		Token:     token,
		UserID:    v.UserID,
		Email:     v.Email,
		Method:    v.Method,
		ExpiresAt: time.Unix(v.ExpiresAt, 0),
	}, nil
}

func (s *SessionStore) Delete(ctx context.Context, token string) error {
	if err := s.client.Del(ctx, sessionKeyPrefix+token).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

var _ repokvstore.SessionRepository = (*SessionStore)(nil)
