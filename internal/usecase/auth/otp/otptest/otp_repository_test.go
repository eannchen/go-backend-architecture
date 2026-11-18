package otptest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp/otptest"
)

// TestOTPRepositoryStoreHistory protects per-call arguments and callback results
// so tests can distinguish repeated storage operations instead of only the last one.
func TestOTPRepositoryStoreHistory(t *testing.T) {
	failure := errors.New("storage unavailable")
	repository := &otptest.OTPRepository{StoreFunc: func(_ context.Context, email, _ string, _ time.Duration) error {
		if email == "second@example.com" {
			return failure
		}
		return nil
	}}
	ctx := t.Context()
	if err := repository.Store(ctx, "first@example.com", "first-hash", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := repository.Store(ctx, "second@example.com", "second-hash", 2*time.Minute); !errors.Is(err, failure) {
		t.Fatalf("Store() = %v, want callback failure", err)
	}
	if len(repository.StoreCalls) != 2 {
		t.Fatalf("calls = %+v, want two", repository.StoreCalls)
	}
	first, second := repository.StoreCalls[0], repository.StoreCalls[1]
	if first.Context != ctx || first.Email != "first@example.com" || first.HashedCode != "first-hash" || first.TTL != time.Minute {
		t.Fatalf("first call = %+v", first)
	}
	if second.Context != ctx || second.Email != "second@example.com" || second.HashedCode != "second-hash" || second.TTL != 2*time.Minute {
		t.Fatalf("second call = %+v", second)
	}
}
