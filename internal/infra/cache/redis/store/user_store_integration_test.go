//go:build integration

package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

func TestUserCacheStoreIntegration(t *testing.T) {
	client := requireRedisTestClient(t)

	ctx := context.Background()
	store := NewUserCacheStore(client, time.Minute)
	id := time.Now().UnixNano()
	key := userKeyPrefix + strconv.FormatInt(id, 10)
	t.Cleanup(func() {
		if err := store.DeleteByID(context.Background(), id); err != nil {
			t.Errorf("cleanup cached user: %v", err)
		}
	})

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
	// TTL is Redis-owned behavior, so a real Redis integration test should prove
	// the expiry was stored rather than only checking the serialized value.
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read cached user TTL: %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("cached user TTL = %v, want within (0, %v]", ttl, time.Minute)
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
