package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

type stubSessionRepo struct {
	createCalls int
	created     repokvstore.SessionData
	createdTTL  time.Duration
	createErr   error

	getResult repokvstore.SessionData
	getErr    error

	deleteCalls int
	deleted     string
	deleteErr   error
}

func (s *stubSessionRepo) Create(_ context.Context, session repokvstore.SessionData, ttl time.Duration) error {
	s.createCalls++
	s.created = session
	s.createdTTL = ttl
	return s.createErr
}

func (s *stubSessionRepo) GetByToken(_ context.Context, _ string) (repokvstore.SessionData, error) {
	return s.getResult, s.getErr
}

func (s *stubSessionRepo) Delete(_ context.Context, token string) error {
	s.deleteCalls++
	s.deleted = token
	return s.deleteErr
}

func TestServerSessionManagerCreateStoresSession(t *testing.T) {
	repo := &stubSessionRepo{}
	ttl := 20 * time.Minute
	mgr := NewServerSessionManager(nil, nil, repo, ttl)

	identity := auth.Identity{
		UserID: 42,
		Email:  "test@example.com",
		Method: auth.MethodOTP,
	}

	got, err := mgr.Create(context.Background(), identity)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Token == "" {
		t.Fatal("expected generated token to be non-empty")
	}
	if repo.createCalls != 1 {
		t.Fatalf("expected one create call, got %d", repo.createCalls)
	}
	if repo.createdTTL != ttl {
		t.Fatalf("expected ttl %v, got %v", ttl, repo.createdTTL)
	}
	if repo.created.Token != got.Token {
		t.Fatalf("expected repository token %q to match returned token %q", repo.created.Token, got.Token)
	}
	if repo.created.UserID != identity.UserID || repo.created.Email != identity.Email || repo.created.Method != string(identity.Method) {
		t.Fatalf("unexpected session data persisted: %+v", repo.created)
	}
	if got.UserID != identity.UserID || got.Email != identity.Email || got.Method != identity.Method {
		t.Fatalf("unexpected returned session: %+v", got)
	}
}

func TestServerSessionManagerValidateRejectsEmptyToken(t *testing.T) {
	mgr := NewServerSessionManager(nil, nil, &stubSessionRepo{}, 15*time.Minute)

	_, err := mgr.Validate(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %T", err)
	}
	if appErr.Code != apperr.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %q", appErr.Code)
	}
}

func TestServerSessionManagerValidateExpiredSessionDeletesToken(t *testing.T) {
	repo := &stubSessionRepo{
		getResult: repokvstore.SessionData{
			Token:     "expired-token",
			UserID:    7,
			Email:     "expired@example.com",
			Method:    string(auth.MethodOTP),
			ExpiresAt: time.Now().Add(-time.Minute),
		},
	}
	mgr := NewServerSessionManager(nil, nil, repo, 15*time.Minute)

	_, err := mgr.Validate(context.Background(), "expired-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %T", err)
	}
	if appErr.Code != apperr.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %q", appErr.Code)
	}
	if repo.deleteCalls != 1 {
		t.Fatalf("expected one delete call for expired token, got %d", repo.deleteCalls)
	}
	if repo.deleted != "expired-token" {
		t.Fatalf("expected deleted token %q, got %q", "expired-token", repo.deleted)
	}
}

func TestServerSessionManagerRevokeWrapsRepositoryError(t *testing.T) {
	repo := &stubSessionRepo{deleteErr: errors.New("redis unavailable")}
	mgr := NewServerSessionManager(nil, nil, repo, 15*time.Minute)

	err := mgr.Revoke(context.Background(), "token-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %T", err)
	}
	if appErr.Code != apperr.CodeInternal {
		t.Fatalf("expected internal code, got %q", appErr.Code)
	}
}
