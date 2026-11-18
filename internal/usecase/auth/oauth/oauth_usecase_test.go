package oauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	oauthtest "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth/oauthtest"
)

// TestOAuthAuthenticatorAuthorize checks authorization creates the state and redirect needed for a safe OAuth flow.
func TestOAuthAuthenticatorAuthorize(t *testing.T) {
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

			stateRepo := &oauthtest.OAuthStateRepository{
				StoreFunc: func(context.Context, string, oauth.OAuthStateData, time.Duration) error {
					return tt.storeErr
				},
			}
			provider := &oauthtest.OAuthProvider{
				NameFunc: func() string { return "google" },
				AuthCodeURLFunc: func(state, _ string) string {
					return "https://oauth.test/authorize?state=" + state
				},
			}
			uc := oauth.NewOAuthAuthenticator(observability.NoopTracer{}, observability.NoopMeter{}, stateRepo, &oauthtest.UserRepository{}, provider)

			got, err := uc.Authorize(context.Background(), tt.provider)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if len(stateRepo.StoreCalls) != tt.wantStoreCalls {
				t.Fatalf("expected store calls %d, got %d", tt.wantStoreCalls, len(stateRepo.StoreCalls))
			}
			if len(provider.AuthCodeURLCalls) != tt.wantAuthCalls {
				t.Fatalf("expected auth url calls %d, got %d", tt.wantAuthCalls, len(provider.AuthCodeURLCalls))
			}
			if tt.wantAuthCalls == 1 {
				if !strings.Contains(got.RedirectURL, stateRepo.StoreCalls[0].State) {
					t.Fatalf("expected redirect URL %q to contain stored state %q", got.RedirectURL, stateRepo.StoreCalls[0].State)
				}
				if got.BrowserBinding == "" || stateRepo.StoreCalls[0].Data.BrowserBindingHash != expectedDigest(got.BrowserBinding) {
					t.Fatalf("expected stored browser-binding hash, got %+v", stateRepo.StoreCalls[0].Data)
				}
			}
		})
	}
}

// TestOAuthAuthenticatorHandleCallback checks callbacks validate state, resolve the user, and create an application session.
func TestOAuthAuthenticatorHandleCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		provider          string
		code              string
		stateValid        bool
		stateErr          error
		exchangeResult    oauth.OAuthUserInfo
		exchangeErr       error
		upsertResult      domainuser.User
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
			name:             "rejects unsupported provider without consuming state",
			provider:         "missing",
			code:             "code-1",
			stateValid:       true,
			wantCode:         apperr.CodeInvalidArgument,
			wantConsumeCalls: 0,
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
			exchangeResult:    oauth.OAuthUserInfo{ProviderUserID: "google-1", Email: "user@example.com"},
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
			exchangeResult:    oauth.OAuthUserInfo{ProviderUserID: "google-1", Email: "user@example.com"},
			upsertResult:      domainuser.User{ID: 77, Email: "user@example.com", Status: domainuser.StatusActive},
			wantIdentity:      auth.Identity{UserID: 77, Email: "user@example.com", Method: auth.MethodOAuth},
			wantConsumeCalls:  1,
			wantExchangeCalls: 1,
			wantUpsertCalls:   1,
		},
		{
			name:              "rejects disabled user",
			provider:          "google",
			code:              "code-1",
			stateValid:        true,
			exchangeResult:    oauth.OAuthUserInfo{ProviderUserID: "google-2", Email: "disabled@example.com"},
			upsertResult:      domainuser.User{ID: 78, Email: "disabled@example.com", Status: domainuser.StatusDisabled},
			wantCode:          apperr.CodeForbidden,
			wantConsumeCalls:  1,
			wantExchangeCalls: 1,
			wantUpsertCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stateRepo := &oauthtest.OAuthStateRepository{
				ConsumeFunc: func(context.Context, string) (oauth.OAuthStateData, bool, error) {
					return oauth.OAuthStateData{BrowserBindingHash: expectedDigest("binding-1")}, tt.stateValid, tt.stateErr
				},
			}
			provider := &oauthtest.OAuthProvider{
				NameFunc: func() string { return "google" },
				ExchangeFunc: func(context.Context, string, string) (oauth.OAuthUserInfo, error) {
					return tt.exchangeResult, tt.exchangeErr
				},
			}
			userRepo := &oauthtest.UserRepository{
				UpsertOAuthUserFunc: func(context.Context, oauth.OAuthUserUpsert) (domainuser.User, error) {
					return tt.upsertResult, tt.upsertErr
				},
			}
			uc := oauth.NewOAuthAuthenticator(observability.NoopTracer{}, observability.NoopMeter{}, stateRepo, userRepo, provider)

			got, err := uc.HandleCallback(context.Background(), tt.provider, tt.code, "state-1", "binding-1")

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tt.wantIdentity {
				t.Fatalf("expected identity %+v, got %+v", tt.wantIdentity, got)
			}
			if len(stateRepo.ConsumeCalls) != tt.wantConsumeCalls {
				t.Fatalf("expected consume calls %d, got %d", tt.wantConsumeCalls, len(stateRepo.ConsumeCalls))
			}
			if len(provider.ExchangeCalls) != tt.wantExchangeCalls {
				t.Fatalf("expected exchange calls %d, got %d", tt.wantExchangeCalls, len(provider.ExchangeCalls))
			}
			if len(userRepo.UpsertOAuthUserCalls) != tt.wantUpsertCalls {
				t.Fatalf("expected upsert calls %d, got %d", tt.wantUpsertCalls, len(userRepo.UpsertOAuthUserCalls))
			}
			if tt.wantUpsertCalls == 1 {
				wantInfo := oauth.OAuthUserUpsert{
					Provider:       tt.provider,
					ProviderUserID: tt.exchangeResult.ProviderUserID,
					Email:          tt.exchangeResult.Email,
				}
				if userRepo.UpsertOAuthUserCalls[0].Info != wantInfo {
					t.Fatalf("expected upsert info %+v, got %+v", wantInfo, userRepo.UpsertOAuthUserCalls[0].Info)
				}
			}
		})
	}
}

// TestOAuthAuthenticatorHandleCallbackRejectsMismatchedBrowserBinding checks a callback from another browser cannot consume the OAuth state.
func TestOAuthAuthenticatorHandleCallbackRejectsMismatchedBrowserBinding(t *testing.T) {
	stateRepo := &oauthtest.OAuthStateRepository{
		ConsumeFunc: func(context.Context, string) (oauth.OAuthStateData, bool, error) {
			return oauth.OAuthStateData{BrowserBindingHash: expectedDigest("expected")}, true, nil
		},
	}
	provider := &oauthtest.OAuthProvider{NameFunc: func() string { return "google" }}
	uc := oauth.NewOAuthAuthenticator(observability.NoopTracer{}, observability.NoopMeter{}, stateRepo, &oauthtest.UserRepository{}, provider)

	_, err := uc.HandleCallback(context.Background(), "google", "code", "state", "wrong")
	assertAppCode(t, err, apperr.CodeUnauthorized)
	if len(provider.ExchangeCalls) != 0 {
		t.Fatalf("Exchange() calls = %d, want 0", len(provider.ExchangeCalls))
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

// expectedDigest matches the SHA-256 representation required by the storage contract.
func expectedDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
