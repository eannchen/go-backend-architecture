package user

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/repository/cache/cachetest"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	"github.com/eannchen/go-backend-architecture/internal/repository/db/dbtest"
)

func TestCachedUserStoreGetByID(t *testing.T) {
	tests := []struct {
		name          string
		cacheUser     repodb.User
		cacheFound    bool
		cacheErr      error
		baseUser      repodb.User
		baseErr       error
		setErr        error
		wantUser      repodb.User
		wantErr       bool
		wantBaseCalls int
		wantSetCalls  int
		wantWarnCalls int
	}{
		{
			name:       "returns cache hit",
			cacheUser:  repodb.User{ID: 1, Email: "cached@example.com"},
			cacheFound: true,
			wantUser:   repodb.User{ID: 1, Email: "cached@example.com"},
		},
		{
			name:          "loads and caches on miss",
			baseUser:      repodb.User{ID: 1, Email: "db@example.com"},
			wantUser:      repodb.User{ID: 1, Email: "db@example.com"},
			wantBaseCalls: 1,
			wantSetCalls:  1,
		},
		{
			name:          "falls back when cache read fails",
			cacheErr:      errors.New("redis get failed"),
			baseUser:      repodb.User{ID: 1, Email: "db@example.com"},
			wantUser:      repodb.User{ID: 1, Email: "db@example.com"},
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
			baseUser:      repodb.User{ID: 1, Email: "db@example.com"},
			setErr:        errors.New("redis set failed"),
			wantUser:      repodb.User{ID: 1, Email: "db@example.com"},
			wantBaseCalls: 1,
			wantSetCalls:  1,
			wantWarnCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &loggertest.Logger{}
			cache := &cachetest.UserCacheStore{
				GetByIDFunc: func(context.Context, int64) (repodb.User, bool, error) {
					return tt.cacheUser, tt.cacheFound, tt.cacheErr
				},
				SetByIDFunc: func(context.Context, int64, repodb.User) error {
					return tt.setErr
				},
			}
			base := &dbtest.UserRepository{
				GetByIDFunc: func(context.Context, int64) (repodb.User, error) {
					return tt.baseUser, tt.baseErr
				},
			}
			store := NewCachedUserStore(log, nil, base, cache)

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
			if base.GetByIDCalls != tt.wantBaseCalls {
				t.Fatalf("expected base get calls %d, got %d", tt.wantBaseCalls, base.GetByIDCalls)
			}
			if cache.SetByIDCalls != tt.wantSetCalls {
				t.Fatalf("expected cache set calls %d, got %d", tt.wantSetCalls, cache.SetByIDCalls)
			}
			if log.WarnCalls != tt.wantWarnCalls {
				t.Fatalf("expected warn calls %d, got %d", tt.wantWarnCalls, log.WarnCalls)
			}
		})
	}
}

func TestCachedUserStoreDelegatesEmailAndCreate(t *testing.T) {
	base := &dbtest.UserRepository{
		GetByEmailFunc: func(context.Context, string) (repodb.User, error) {
			return repodb.User{ID: 2, Email: "user@example.com"}, nil
		},
		CreateByEmailFunc: func(context.Context, string) (repodb.User, error) {
			return repodb.User{ID: 3, Email: "new@example.com"}, nil
		},
	}
	cache := &cachetest.UserCacheStore{}
	store := NewCachedUserStore(&loggertest.Logger{}, nil, base, cache)

	gotEmail, err := store.GetByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if gotEmail.ID != 2 || base.GetByEmailCalls != 1 || cache.GetByIDCalls != 0 {
		t.Fatalf("unexpected GetByEmail behavior: user=%+v base=%d cache=%d", gotEmail, base.GetByEmailCalls, cache.GetByIDCalls)
	}

	gotCreate, err := store.CreateByEmail(context.Background(), "new@example.com")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if gotCreate.ID != 3 || base.CreateByEmailCalls != 1 || cache.SetByIDCalls != 0 {
		t.Fatalf("unexpected CreateByEmail behavior: user=%+v base=%d cache=%d", gotCreate, base.CreateByEmailCalls, cache.SetByIDCalls)
	}
}

func TestCachedUserStoreUpsertOAuthUser(t *testing.T) {
	tests := []struct {
		name            string
		baseUser        repodb.User
		baseErr         error
		deleteErr       error
		wantUser        repodb.User
		wantErr         bool
		wantDeleteCalls int
		wantWarnCalls   int
	}{
		{
			name:            "invalidates cache after upsert",
			baseUser:        repodb.User{ID: 9, Email: "oauth@example.com"},
			wantUser:        repodb.User{ID: 9, Email: "oauth@example.com"},
			wantDeleteCalls: 1,
		},
		{
			name:            "logs cache invalidation failure and returns user",
			baseUser:        repodb.User{ID: 9, Email: "oauth@example.com"},
			deleteErr:       errors.New("redis del failed"),
			wantUser:        repodb.User{ID: 9, Email: "oauth@example.com"},
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
			log := &loggertest.Logger{}
			base := &dbtest.UserRepository{
				UpsertOAuthUserFunc: func(context.Context, repodb.OAuthUserUpsert) (repodb.User, error) {
					return tt.baseUser, tt.baseErr
				},
			}
			cache := &cachetest.UserCacheStore{
				DeleteByIDFunc: func(context.Context, int64) error {
					return tt.deleteErr
				},
			}
			store := NewCachedUserStore(log, nil, base, cache)

			got, err := store.UpsertOAuthUser(context.Background(), repodb.OAuthUserUpsert{
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
			if cache.DeleteByIDCalls != tt.wantDeleteCalls {
				t.Fatalf("expected delete calls %d, got %d", tt.wantDeleteCalls, cache.DeleteByIDCalls)
			}
			if log.WarnCalls != tt.wantWarnCalls {
				t.Fatalf("expected warn calls %d, got %d", tt.wantWarnCalls, log.WarnCalls)
			}
		})
	}
}
