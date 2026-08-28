//go:build integration

package store

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

func TestUserStoreIntegration(t *testing.T) {
	pool := requirePostgresTestPool(t)

	store := NewUserStore(pool, observability.NoopTracer{})
	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "user-store-" + suffix + "@example.com"
	oauthEmail := "oauth-" + suffix + "@example.com"
	providerUserID := "provider-user-" + suffix
	// The container is shared by every test in this package, so each test owns
	// cleanup for the rows it creates. t.Cleanup runs even when an assertion fails.
	t.Cleanup(func() { cleanupUsersByEmail(t, pool, email, oauthEmail) })

	created, err := store.CreateByEmail(ctx, email)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.ID == 0 || created.Email != email {
		t.Fatalf("unexpected created user: %+v", created)
	}

	byEmail, err := store.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if byEmail != created {
		t.Fatalf("expected email lookup %+v, got %+v", created, byEmail)
	}

	byID, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if byID != created {
		t.Fatalf("expected id lookup %+v, got %+v", created, byID)
	}

	if _, err := store.CreateByEmail(ctx, email); !errors.Is(err, repodb.ErrDuplicateKey) {
		t.Fatalf("expected duplicate key error, got %v", err)
	}

	oauthUser, err := store.UpsertOAuthUser(ctx, repodb.OAuthUserUpsert{
		Provider:       "google",
		ProviderUserID: providerUserID,
		Email:          oauthEmail,
	})
	if err != nil {
		t.Fatalf("upsert oauth user: %v", err)
	}
	if oauthUser.ID == 0 || oauthUser.Email != oauthEmail {
		t.Fatalf("unexpected oauth user: %+v", oauthUser)
	}

	oauthAgain, err := store.UpsertOAuthUser(ctx, repodb.OAuthUserUpsert{
		Provider:       "google",
		ProviderUserID: providerUserID,
		Email:          "ignored@example.com",
	})
	if err != nil {
		t.Fatalf("upsert same oauth user: %v", err)
	}
	if oauthAgain != oauthUser {
		t.Fatalf("expected existing oauth user %+v, got %+v", oauthUser, oauthAgain)
	}
}

func TestUserStoreIntegration_MissingUserMapsNotFound(t *testing.T) {
	store := NewUserStore(requirePostgresTestPool(t), observability.NoopTracer{})
	ctx := context.Background()
	missingEmail := "missing-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"

	if _, err := store.GetByEmail(ctx, missingEmail); !errors.Is(err, repodb.ErrNotFound) {
		t.Fatalf("GetByEmail error = %v, want ErrNotFound", err)
	}
	if _, err := store.GetByID(ctx, -1); !errors.Is(err, repodb.ErrNotFound) {
		t.Fatalf("GetByID error = %v, want ErrNotFound", err)
	}
}

func cleanupUsersByEmail(t *testing.T, pool *pgxpool.Pool, email, oauthEmail string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE email IN ($1, $2)", email, oauthEmail); err != nil {
		t.Fatalf("cleanup test users: %v", err)
	}
}
