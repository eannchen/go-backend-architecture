package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/repository/kvstore/kvstoretest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// TestServerSessionManagerCreate checks session creation stores the session and maps dependency failures.
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
			mgr := NewServerSessionManager(nil, nil, nil, repo, ttl)
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
				if repo.CreateTTL != ttl {
					t.Fatalf("expected ttl %v, got %v", ttl, repo.CreateTTL)
				}
				if repo.CreateSession.Token != got.Token {
					t.Fatalf("expected repository token %q to match returned token %q", repo.CreateSession.Token, got.Token)
				}
				if repo.CreateSession.UserID != identity.UserID || repo.CreateSession.Email != identity.Email || repo.CreateSession.Method != string(identity.Method) {
					t.Fatalf("unexpected session data persisted: %+v", repo.CreateSession)
				}
				if got.UserID != identity.UserID || got.Email != identity.Email || got.Method != identity.Method {
					t.Fatalf("unexpected returned session: %+v", got)
				}
			}
		})
	}
}

// TestServerSessionManagerValidate checks validation accepts usable sessions and rejects missing or expired ones.
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
			name:         "missing stored session",
			token:        "missing-token",
			getErr:       repokvstore.ErrSessionNotFound,
			wantCode:     apperr.CodeUnauthorized,
			wantGetCalls: 1,
		},
		{
			name:         "store unavailable",
			token:        "existing-token",
			getErr:       errors.New("redis unavailable"),
			wantCode:     apperr.CodeUnavailable,
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
				DeleteFunc: func(context.Context, string) error { return nil },
			}
			mgr := NewServerSessionManager(nil, nil, nil, repo, 15*time.Minute)

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

// TestServerSessionManagerValidateLogsExpiredSessionCleanupFailure confirms that
// failed cleanup is visible without exposing the token or accepting an expired session.
func TestServerSessionManagerValidateLogsExpiredSessionCleanupFailure(t *testing.T) {
	token := "private-session-token"
	deleteErr := errors.New("redis unavailable")
	repo := &kvstoretest.SessionRepository{
		GetByTokenFunc: func(context.Context, string) (repokvstore.SessionData, error) {
			return repokvstore.SessionData{Token: token, ExpiresAt: time.Now().Add(-time.Minute)}, nil
		},
		DeleteFunc: func(context.Context, string) error { return deleteErr },
	}
	log := &loggertest.Logger{WarnFunc: func(context.Context, string, ...logger.Fields) {}}
	mgr := NewServerSessionManager(nil, log, nil, repo, time.Minute)

	_, err := mgr.Validate(context.Background(), token)

	assertAppCode(t, err, apperr.CodeUnauthorized)
	if repo.DeleteCalls != 1 || repo.DeleteToken != token {
		t.Fatalf("delete calls = %d, token = %q; want one cleanup attempt", repo.DeleteCalls, repo.DeleteToken)
	}
	if len(log.WarnCalls) != 1 {
		t.Fatalf("warning calls = %d, want 1", len(log.WarnCalls))
	}
	warning := log.WarnCalls[0]
	if !strings.Contains(warning.Message, "session") || !strings.Contains(warning.Message, "cleanup") || strings.Contains(warning.Message, token) {
		t.Fatalf("warning message = %q, want session cleanup context without token", warning.Message)
	}
	if len(warning.Fields) != 1 {
		t.Fatalf("warning field groups = %d, want 1", len(warning.Fields))
	}
	fields := warning.Fields[0]
	if len(fields) != 1 {
		t.Fatalf("warning fields = %#v, want only the cleanup error", fields)
	}
	gotErr, ok := fields["error"].(error)
	if !ok || !errors.Is(gotErr, deleteErr) {
		t.Fatalf("warning error = %v, want %v", gotErr, deleteErr)
	}
}

// TestServerSessionManagerRevoke checks revocation handles existing and missing sessions with the intended error mapping.
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
			mgr := NewServerSessionManager(nil, nil, nil, repo, 15*time.Minute)

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
