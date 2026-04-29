package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	"github.com/eannchen/go-backend-architecture/internal/repository/db/dbtest"
	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/oauth"
	"github.com/eannchen/go-backend-architecture/internal/repository/external/oauth/oauthtest"
	"github.com/eannchen/go-backend-architecture/internal/repository/kvstore/kvstoretest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

func TestOAuthAuthenticatorAuthorizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		provider       string
		storeErr       error
		wantCode       apperr.Code
		wantStoreCalls int
		wantAuthCalls  int
	}{
		{
			name:     "rejects unsupported provider",
			provider: "missing",
			wantCode: apperr.CodeInvalidArgument,
		},
		{
			name:           "wraps state store failure",
			provider:       "google",
			storeErr:       errors.New("redis unavailable"),
			wantCode:       apperr.CodeInternal,
			wantStoreCalls: 1,
		},
		{
			name:           "stores state and returns provider URL",
			provider:       "google",
			wantStoreCalls: 1,
			wantAuthCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stateRepo := &kvstoretest.OAuthStateRepository{
				StoreFunc: func(context.Context, string, time.Duration) error {
					return tt.storeErr
				},
			}
			provider := &oauthtest.OAuthProvider{ProviderName: "google"}
			uc := NewOAuthAuthenticator(nil, nil, stateRepo, &dbtest.UserRepository{}, provider)

			got, err := uc.AuthorizeURL(context.Background(), tt.provider)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if stateRepo.StoreCalls != tt.wantStoreCalls {
				t.Fatalf("expected store calls %d, got %d", tt.wantStoreCalls, stateRepo.StoreCalls)
			}
			if provider.AuthCodeCalls != tt.wantAuthCalls {
				t.Fatalf("expected auth url calls %d, got %d", tt.wantAuthCalls, provider.AuthCodeCalls)
			}
			if tt.wantAuthCalls == 1 && !strings.Contains(got, stateRepo.StoredState) {
				t.Fatalf("expected redirect URL %q to contain stored state %q", got, stateRepo.StoredState)
			}
		})
	}
}

func TestOAuthAuthenticatorHandleCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		provider          string
		code              string
		stateValid        bool
		stateErr          error
		exchangeResult    repoexternal.OAuthUserInfo
		exchangeErr       error
		upsertResult      repodb.User
		upsertErr         error
		wantIdentity      auth.Identity
		wantCode          apperr.Code
		wantConsumeCalls  int
		wantExchangeCalls int
		wantUpsertCalls   int
	}{
		{
			name:             "wraps state consume failure",
			provider:         "google",
			code:             "code-1",
			stateErr:         errors.New("redis unavailable"),
			wantCode:         apperr.CodeInternal,
			wantConsumeCalls: 1,
		},
		{
			name:             "rejects invalid state",
			provider:         "google",
			code:             "code-1",
			wantCode:         apperr.CodeUnauthorized,
			wantConsumeCalls: 1,
		},
		{
			name:             "rejects unsupported provider after state validation",
			provider:         "missing",
			code:             "code-1",
			stateValid:       true,
			wantCode:         apperr.CodeInvalidArgument,
			wantConsumeCalls: 1,
		},
		{
			name:              "maps exchange timeout",
			provider:          "google",
			code:              "code-1",
			stateValid:        true,
			exchangeErr:       context.DeadlineExceeded,
			wantCode:          apperr.CodeTimeout,
			wantConsumeCalls:  1,
			wantExchangeCalls: 1,
		},
		{
			name:              "maps exchange failure to unauthorized",
			provider:          "google",
			code:              "code-1",
			stateValid:        true,
			exchangeErr:       errors.New("bad code"),
			wantCode:          apperr.CodeUnauthorized,
			wantConsumeCalls:  1,
			wantExchangeCalls: 1,
		},
		{
			name:              "wraps user upsert failure",
			provider:          "google",
			code:              "code-1",
			stateValid:        true,
			exchangeResult:    repoexternal.OAuthUserInfo{ProviderUserID: "google-1", Email: "user@example.com"},
			upsertErr:         errors.New("postgres unavailable"),
			wantCode:          apperr.CodeInternal,
			wantConsumeCalls:  1,
			wantExchangeCalls: 1,
			wantUpsertCalls:   1,
		},
		{
			name:              "returns oauth identity",
			provider:          "google",
			code:              "code-1",
			stateValid:        true,
			exchangeResult:    repoexternal.OAuthUserInfo{ProviderUserID: "google-1", Email: "user@example.com"},
			upsertResult:      repodb.User{ID: 77, Email: "user@example.com"},
			wantIdentity:      auth.Identity{UserID: 77, Email: "user@example.com", Method: auth.MethodOAuth},
			wantConsumeCalls:  1,
			wantExchangeCalls: 1,
			wantUpsertCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stateRepo := &kvstoretest.OAuthStateRepository{
				ConsumeFunc: func(context.Context, string) (bool, error) {
					return tt.stateValid, tt.stateErr
				},
			}
			provider := &oauthtest.OAuthProvider{
				ProviderName: "google",
				ExchangeFunc: func(context.Context, string) (repoexternal.OAuthUserInfo, error) {
					return tt.exchangeResult, tt.exchangeErr
				},
			}
			userRepo := &dbtest.UserRepository{
				UpsertOAuthUserFunc: func(context.Context, repodb.OAuthUserUpsert) (repodb.User, error) {
					return tt.upsertResult, tt.upsertErr
				},
			}
			uc := NewOAuthAuthenticator(nil, nil, stateRepo, userRepo, provider)

			got, err := uc.HandleCallback(context.Background(), tt.provider, tt.code, "state-1")

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tt.wantIdentity {
				t.Fatalf("expected identity %+v, got %+v", tt.wantIdentity, got)
			}
			if stateRepo.ConsumeCalls != tt.wantConsumeCalls {
				t.Fatalf("expected consume calls %d, got %d", tt.wantConsumeCalls, stateRepo.ConsumeCalls)
			}
			if provider.ExchangeCalls != tt.wantExchangeCalls {
				t.Fatalf("expected exchange calls %d, got %d", tt.wantExchangeCalls, provider.ExchangeCalls)
			}
			if userRepo.UpsertOAuthUserCalls != tt.wantUpsertCalls {
				t.Fatalf("expected upsert calls %d, got %d", tt.wantUpsertCalls, userRepo.UpsertOAuthUserCalls)
			}
			if tt.wantUpsertCalls == 1 {
				wantInfo := repodb.OAuthUserUpsert{
					Provider:       tt.provider,
					ProviderUserID: tt.exchangeResult.ProviderUserID,
					Email:          tt.exchangeResult.Email,
				}
				if userRepo.UpsertOAuthUserInfo != wantInfo {
					t.Fatalf("expected upsert info %+v, got %+v", wantInfo, userRepo.UpsertOAuthUserInfo)
				}
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
