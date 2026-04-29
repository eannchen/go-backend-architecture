package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/repository/kvstore/kvstoretest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

func TestServerSessionManagerCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		repoErr         error
		wantCode        apperr.Code
		wantCreateCalls int
	}{
		{
			name:            "stores session",
			wantCreateCalls: 1,
		},
		{
			name:            "wraps repository failure",
			repoErr:         errors.New("redis unavailable"),
			wantCode:        apperr.CodeInternal,
			wantCreateCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &kvstoretest.SessionRepository{
				CreateFunc: func(context.Context, repokvstore.SessionData, time.Duration) error {
					return tt.repoErr
				},
			}
			ttl := 20 * time.Minute
			mgr := NewServerSessionManager(nil, nil, repo, ttl)
			identity := auth.Identity{UserID: 42, Email: "test@example.com", Method: auth.MethodOTP}

			got, err := mgr.Create(context.Background(), identity)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if repo.CreateCalls != tt.wantCreateCalls {
				t.Fatalf("expected create calls %d, got %d", tt.wantCreateCalls, repo.CreateCalls)
			}
			if tt.wantCode == "" {
				if got.Token == "" {
					t.Fatal("expected generated token to be non-empty")
				}
				if repo.CreatedTTL != ttl {
					t.Fatalf("expected ttl %v, got %v", ttl, repo.CreatedTTL)
				}
				if repo.Created.Token != got.Token {
					t.Fatalf("expected repository token %q to match returned token %q", repo.Created.Token, got.Token)
				}
				if repo.Created.UserID != identity.UserID || repo.Created.Email != identity.Email || repo.Created.Method != string(identity.Method) {
					t.Fatalf("unexpected session data persisted: %+v", repo.Created)
				}
				if got.UserID != identity.UserID || got.Email != identity.Email || got.Method != identity.Method {
					t.Fatalf("unexpected returned session: %+v", got)
				}
			}
		})
	}
}

func TestServerSessionManagerValidate(t *testing.T) {
	t.Parallel()

	validSession := repokvstore.SessionData{
		Token:     "token-1",
		UserID:    7,
		Email:     "user@example.com",
		Method:    string(auth.MethodOTP),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	tests := []struct {
		name            string
		token           string
		getResult       repokvstore.SessionData
		getErr          error
		wantSession     auth.Session
		wantCode        apperr.Code
		wantGetCalls    int
		wantDeleteCalls int
	}{
		{
			name:     "rejects empty token",
			wantCode: apperr.CodeUnauthorized,
		},
		{
			name:         "wraps missing token",
			token:        "missing-token",
			getErr:       errors.New("not found"),
			wantCode:     apperr.CodeUnauthorized,
			wantGetCalls: 1,
		},
		{
			name:            "deletes expired session",
			token:           "expired-token",
			getResult:       repokvstore.SessionData{Token: "expired-token", UserID: 7, Email: "expired@example.com", Method: string(auth.MethodOTP), ExpiresAt: time.Now().Add(-time.Minute)},
			wantCode:        apperr.CodeUnauthorized,
			wantGetCalls:    1,
			wantDeleteCalls: 1,
		},
		{
			name:         "returns active session",
			token:        "token-1",
			getResult:    validSession,
			wantSession:  auth.Session{Token: "token-1", UserID: 7, Email: "user@example.com", Method: auth.MethodOTP, ExpiresAt: validSession.ExpiresAt},
			wantGetCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &kvstoretest.SessionRepository{
				GetByTokenFunc: func(context.Context, string) (repokvstore.SessionData, error) {
					return tt.getResult, tt.getErr
				},
			}
			mgr := NewServerSessionManager(nil, nil, repo, 15*time.Minute)

			got, err := mgr.Validate(context.Background(), tt.token)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tt.wantSession {
				t.Fatalf("expected session %+v, got %+v", tt.wantSession, got)
			}
			if repo.GetByTokenCalls != tt.wantGetCalls {
				t.Fatalf("expected get calls %d, got %d", tt.wantGetCalls, repo.GetByTokenCalls)
			}
			if repo.DeleteCalls != tt.wantDeleteCalls {
				t.Fatalf("expected delete calls %d, got %d", tt.wantDeleteCalls, repo.DeleteCalls)
			}
		})
	}
}

func TestServerSessionManagerRevoke(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		repoErr  error
		wantCode apperr.Code
	}{
		{name: "deletes token"},
		{name: "wraps repository error", repoErr: errors.New("redis unavailable"), wantCode: apperr.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &kvstoretest.SessionRepository{
				DeleteFunc: func(context.Context, string) error {
					return tt.repoErr
				},
			}
			mgr := NewServerSessionManager(nil, nil, repo, 15*time.Minute)

			err := mgr.Revoke(context.Background(), "token-1")

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if repo.DeleteCalls != 1 {
				t.Fatalf("expected one delete call, got %d", repo.DeleteCalls)
			}
			if repo.DeleteToken != "token-1" {
				t.Fatalf("expected deleted token %q, got %q", "token-1", repo.DeleteToken)
			}
		})
	}
}

func assertAppCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected app error %q, got nil", want)
	}
	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %T", err)
	}
	if appErr.Code != want {
		t.Fatalf("expected code %q, got %q", want, appErr.Code)
	}
}
