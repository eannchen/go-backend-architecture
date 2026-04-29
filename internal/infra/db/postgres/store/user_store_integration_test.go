//go:build integration

package store

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

func TestUserStoreIntegration(t *testing.T) {
	pool := openPostgresPool(t)
	defer pool.Close()
	requireUserSchema(t, pool)

	store := NewUserStore(pool, observability.NoopTracer{})
	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "user-store-" + suffix + "@example.com"
	oauthEmail := "oauth-" + suffix + "@example.com"
	providerUserID := "provider-user-" + suffix
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

func openPostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		t.Skip("set DB_URL to run postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skip: unable to create postgres pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skip: postgres unavailable: %v", err)
	}
	return pool
}

func requireUserSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, "SELECT 1 FROM users LIMIT 1"); err != nil {
		t.Skipf("skip: users schema is not migrated: %v", err)
	}
}

func cleanupUsersByEmail(t *testing.T, pool *pgxpool.Pool, emails ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE email IN ($1, $2)", emails[0], emails[1]); err != nil {
		t.Fatalf("cleanup test users: %v", err)
	}
}
