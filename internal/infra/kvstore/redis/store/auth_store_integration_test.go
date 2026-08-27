//go:build integration

package store

import (
	"context"
	"errors"
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

func TestOTPStoreIntegration(t *testing.T) {
	client := requireRedisTestClient(t)

	store := NewOTPStore(client)
	ctx := context.Background()
	email := "otp-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
	t.Cleanup(func() {
		if err := store.Delete(context.Background(), email); err != nil {
			t.Errorf("cleanup OTP: %v", err)
		}
	})

	if err := store.Store(ctx, email, "hashed-code", time.Minute); err != nil {
		t.Fatalf("store otp: %v", err)
	}
	requirePositiveRedisTTL(t, otpKeyPrefix+email, time.Minute)
	matched, err := store.Consume(ctx, email, "wrong-hash")
	if err != nil {
		t.Fatalf("consume otp: %v", err)
	}
	if matched {
		t.Fatal("expected mismatched OTP hash to leave the code available")
	}

	matched, err = store.Consume(ctx, email, "hashed-code")
	if err != nil {
		t.Fatalf("consume matching otp: %v", err)
	}
	if !matched {
		t.Fatal("expected matching OTP hash to consume the code")
	}
	if _, err := store.Consume(ctx, email, "hashed-code"); !errors.Is(err, repokvstore.ErrOTPNotFound) {
		t.Fatalf("second consume error = %v, want ErrOTPNotFound", err)
	}
}

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
