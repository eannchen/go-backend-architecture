package user

import (
	"context"
	"errors"
	"testing"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	usertest "github.com/eannchen/go-backend-architecture/internal/infra/composed/user/usertest"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	"github.com/eannchen/go-backend-architecture/pkg/logger"
	"github.com/eannchen/go-backend-architecture/pkg/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

// TestCachedUserStoreGetByID checks cache hits, misses, and failures follow the user lookup policy.
func TestCachedUserStoreGetByID(t *testing.T) {
	tests := []struct {
		name          string
		cacheUser     domainuser.User
		cacheFound    bool
		cacheErr      error
		baseUser      domainuser.User
		baseErr       error
		setErr        error
		wantUser      domainuser.User
		wantErr       bool
		wantBaseCalls int
		wantSetCalls  int
		wantWarnCalls int
	}{
		{
			name:       "returns cache hit",
			cacheUser:  domainuser.User{ID: 1, Email: "cached@example.com"},
			cacheFound: true,
			wantUser:   domainuser.User{ID: 1, Email: "cached@example.com"},
		},
		{
			name:          "loads and caches on miss",
			baseUser:      domainuser.User{ID: 1, Email: "db@example.com"},
			wantUser:      domainuser.User{ID: 1, Email: "db@example.com"},
			wantBaseCalls: 1,
			wantSetCalls:  1,
		},
		{
			name:          "falls back when cache read fails",
			cacheErr:      errors.New("redis get failed"),
			baseUser:      domainuser.User{ID: 1, Email: "db@example.com"},
			wantUser:      domainuser.User{ID: 1, Email: "db@example.com"},
			wantBaseCalls: 1,
			wantSetCalls:  1,
			wantWarnCalls: 1,
		},
		{
			name:          "returns base error",
			baseErr:       errors.New("postgres failed"),
			wantErr:       true,
			wantBaseCalls: 1,
		},
		{
			name:          "logs cache write failure and returns user",
			baseUser:      domainuser.User{ID: 1, Email: "db@example.com"},
			setErr:        errors.New("redis set failed"),
			wantUser:      domainuser.User{ID: 1, Email: "db@example.com"},
			wantBaseCalls: 1,
			wantSetCalls:  1,
			wantWarnCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &loggertest.Logger{
				WarnFunc: func(context.Context, string, ...logger.Fields) {},
			}
			cache := &usertest.UserCache{
				GetByIDFunc: func(context.Context, int64) (domainuser.User, bool, error) {
					return tt.cacheUser, tt.cacheFound, tt.cacheErr
				},
				SetByIDFunc: func(context.Context, int64, domainuser.User) error {
					return tt.setErr
				},
			}
			base := &usertest.UserStore{
				GetByIDFunc: func(context.Context, int64) (domainuser.User, error) {
					return tt.baseUser, tt.baseErr
				},
			}
			store := NewCachedUserStore(log, observability.NoopTracer{}, base, cache)

			got, err := store.GetByID(context.Background(), 1)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tt.wantUser {
				t.Fatalf("expected user %+v, got %+v", tt.wantUser, got)
			}
			if len(base.GetByIDCalls) != tt.wantBaseCalls {
				t.Fatalf("expected base get calls %d, got %d", tt.wantBaseCalls, len(base.GetByIDCalls))
			}
			if len(cache.SetByIDCalls) != tt.wantSetCalls {
				t.Fatalf("expected cache set calls %d, got %d", tt.wantSetCalls, len(cache.SetByIDCalls))
			}
			if len(log.WarnCalls) != tt.wantWarnCalls {
				t.Fatalf("expected warn calls %d, got %d", tt.wantWarnCalls, len(log.WarnCalls))
			}
		})
	}
}

// TestCachedUserStoreDelegatesEmailAndCreate checks uncached email lookup and create operations reach the backing repository.
func TestCachedUserStoreDelegatesEmailAndCreate(t *testing.T) {
	base := &usertest.UserStore{
		GetByEmailFunc: func(context.Context, string) (domainuser.User, error) {
			return domainuser.User{ID: 2, Email: "user@example.com"}, nil
		},
		CreateByEmailFunc: func(context.Context, string) (domainuser.User, error) {
			return domainuser.User{ID: 3, Email: "new@example.com"}, nil
		},
	}
	cache := &usertest.UserCache{}
	store := NewCachedUserStore(logger.NoopLogger{}, observability.NoopTracer{}, base, cache)

	gotEmail, err := store.GetByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if gotEmail.ID != 2 || len(base.GetByEmailCalls) != 1 || len(cache.GetByIDCalls) != 0 {
		t.Fatalf("unexpected GetByEmail behavior: user=%+v base=%d cache=%d", gotEmail, len(base.GetByEmailCalls), len(cache.GetByIDCalls))
	}

	gotCreate, err := store.CreateByEmail(context.Background(), "new@example.com")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if gotCreate.ID != 3 || len(base.CreateByEmailCalls) != 1 || len(cache.SetByIDCalls) != 0 {
		t.Fatalf("unexpected CreateByEmail behavior: user=%+v base=%d cache=%d", gotCreate, len(base.CreateByEmailCalls), len(cache.SetByIDCalls))
	}
}

// TestCachedUserStoreUpsertOAuthUser checks OAuth upserts keep the backing store and cache behavior consistent.
func TestCachedUserStoreUpsertOAuthUser(t *testing.T) {
	tests := []struct {
		name            string
		baseUser        domainuser.User
		baseErr         error
		deleteErr       error
		wantUser        domainuser.User
		wantErr         bool
		wantDeleteCalls int
		wantWarnCalls   int
	}{
		{
			name:            "invalidates cache after upsert",
			baseUser:        domainuser.User{ID: 9, Email: "oauth@example.com"},
			wantUser:        domainuser.User{ID: 9, Email: "oauth@example.com"},
			wantDeleteCalls: 1,
		},
		{
			name:            "logs cache invalidation failure and returns user",
			baseUser:        domainuser.User{ID: 9, Email: "oauth@example.com"},
			deleteErr:       errors.New("redis del failed"),
			wantUser:        domainuser.User{ID: 9, Email: "oauth@example.com"},
			wantDeleteCalls: 1,
			wantWarnCalls:   1,
		},
		{
			name:    "returns base error",
			baseErr: errors.New("postgres failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &loggertest.Logger{
				WarnFunc: func(context.Context, string, ...logger.Fields) {},
			}
			base := &usertest.UserStore{
				UpsertOAuthUserFunc: func(context.Context, authoauth.OAuthUserUpsert) (domainuser.User, error) {
					return tt.baseUser, tt.baseErr
				},
			}
			cache := &usertest.UserCache{
				DeleteByIDFunc: func(context.Context, int64) error {
					return tt.deleteErr
				},
			}
			store := NewCachedUserStore(log, observability.NoopTracer{}, base, cache)

			got, err := store.UpsertOAuthUser(context.Background(), authoauth.OAuthUserUpsert{
				Provider: "google", ProviderUserID: "google-1", Email: "oauth@example.com",
			})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tt.wantUser {
				t.Fatalf("expected user %+v, got %+v", tt.wantUser, got)
			}
			if len(cache.DeleteByIDCalls) != tt.wantDeleteCalls {
				t.Fatalf("expected delete calls %d, got %d", tt.wantDeleteCalls, len(cache.DeleteByIDCalls))
			}
			if len(log.WarnCalls) != tt.wantWarnCalls {
				t.Fatalf("expected warn calls %d, got %d", tt.wantWarnCalls, len(log.WarnCalls))
			}
		})
	}
}
