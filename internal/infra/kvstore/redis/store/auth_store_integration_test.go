//go:build integration

package store

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

func TestSessionStoreIntegration(t *testing.T) {
	client := openRedisClient(t)
	defer client.Close()

	store := NewSessionStore(client)
	ctx := context.Background()
	token := "test-session-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = store.Delete(context.Background(), token) })

	want := repokvstore.SessionData{
		Token:     token,
		UserID:    42,
		Email:     "session@example.com",
		Method:    "otp",
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Second),
	}
	if err := store.Create(ctx, want, time.Minute); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := store.GetByToken(ctx, token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got != want {
		t.Fatalf("expected session %+v, got %+v", want, got)
	}

	if err := store.Delete(ctx, token); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := store.GetByToken(ctx, token); err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestOTPStoreIntegration(t *testing.T) {
	client := openRedisClient(t)
	defer client.Close()

	store := NewOTPStore(client)
	ctx := context.Background()
	email := "otp-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
	t.Cleanup(func() { _ = store.Delete(context.Background(), email) })

	if err := store.Store(ctx, email, "hashed-code", time.Minute); err != nil {
		t.Fatalf("store otp: %v", err)
	}
	got, err := store.Get(ctx, email)
	if err != nil {
		t.Fatalf("get otp: %v", err)
	}
	if got != "hashed-code" {
		t.Fatalf("expected hashed-code, got %q", got)
	}
	if err := store.Delete(ctx, email); err != nil {
		t.Fatalf("delete otp: %v", err)
	}
	if _, err := store.Get(ctx, email); err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestOAuthStateStoreIntegration(t *testing.T) {
	client := openRedisClient(t)
	defer client.Close()

	store := NewOAuthStateStore(client)
	ctx := context.Background()
	state := "state-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _, _ = store.Consume(context.Background(), state) })

	if err := store.Store(ctx, state, time.Minute); err != nil {
		t.Fatalf("store state: %v", err)
	}
	ok, err := store.Consume(ctx, state)
	if err != nil {
		t.Fatalf("consume state: %v", err)
	}
	if !ok {
		t.Fatal("expected first consume to find state")
	}
	ok, err = store.Consume(ctx, state)
	if err != nil {
		t.Fatalf("consume state again: %v", err)
	}
	if ok {
		t.Fatal("expected second consume to miss state")
	}
}

func openRedisClient(t *testing.T) *goredis.Client {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("set REDIS_ADDR to run redis integration tests")
	}
	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			t.Skipf("skip: invalid REDIS_DB=%q: %v", v, err)
		}
		db = parsed
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		t.Skipf("skip: redis unavailable: %v", err)
	}
	return client
}
