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
