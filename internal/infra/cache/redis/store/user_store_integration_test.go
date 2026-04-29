//go:build integration

package store

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

func TestUserCacheStoreIntegration(t *testing.T) {
	client := openRedisClient(t)
	defer client.Close()

	ctx := context.Background()
	store := NewUserCacheStore(client, time.Minute)
	id := time.Now().UnixNano()
	t.Cleanup(func() { _ = store.DeleteByID(context.Background(), id) })

	user := repodb.User{ID: id, Email: "cache@example.com"}
	foundUser, found, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get missing user: %v", err)
	}
	if found || foundUser != (repodb.User{}) {
		t.Fatalf("expected cache miss, got found=%v user=%+v", found, foundUser)
	}

	if err := store.SetByID(ctx, id, user); err != nil {
		t.Fatalf("set user: %v", err)
	}
	got, found, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get cached user: %v", err)
	}
	if !found || got != user {
		t.Fatalf("expected cached user %+v, got found=%v user=%+v", user, found, got)
	}

	if err := store.DeleteByID(ctx, id); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	_, found, err = store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if found {
		t.Fatal("expected cache miss after delete")
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
