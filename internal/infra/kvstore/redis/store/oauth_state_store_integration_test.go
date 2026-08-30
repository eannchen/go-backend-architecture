//go:build integration

package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

func TestOAuthStateStoreIntegration(t *testing.T) {
	client := requireRedisTestClient(t)

	store := NewOAuthStateStore(client)
	ctx := context.Background()
	state := "state-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	key := oauthStateKeyPrefix + state
	t.Cleanup(func() { cleanupRedisKeys(t, key) })
	want := repokvstore.OAuthStateData{BrowserBindingHash: "browser-binding-hash"}

	if err := store.Store(ctx, state, want, time.Minute); err != nil {
		t.Fatalf("store state: %v", err)
	}
	requirePositiveRedisTTL(t, key, time.Minute)
	got, found, err := store.Consume(ctx, state)
	if err != nil {
		t.Fatalf("consume state: %v", err)
	}
	if !found || got != want {
		t.Fatalf("first consume = %+v, %v; want %+v, true", got, found, want)
	}
	_, found, err = store.Consume(ctx, state)
	if err != nil {
		t.Fatalf("consume state again: %v", err)
	}
	if found {
		t.Fatal("expected second consume to miss state")
	}
}
