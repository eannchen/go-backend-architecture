//go:build integration

package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

func TestSessionStoreIntegration(t *testing.T) {
	client := requireRedisTestClient(t)

	store := NewSessionStore(client)
	ctx := context.Background()
	token := "test-session-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() {
		if err := store.Delete(context.Background(), token); err != nil {
			t.Errorf("cleanup session: %v", err)
		}
	})

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
	requirePositiveRedisTTL(t, sessionKeyPrefix+token, time.Minute)

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
