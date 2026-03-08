package store

import "testing"

func TestAccountSummaryByIDKey_Deterministic(t *testing.T) {
	keyA := accountSummaryByIDKey(50)
	keyB := accountSummaryByIDKey(50)
	if keyA != keyB {
		t.Fatalf("expected deterministic key, got %q and %q", keyA, keyB)
	}
}

func TestAccountSummaryByIDKey_EncodesValue(t *testing.T) {
	key := accountSummaryByIDKey(10)
	if key != "account_summary:id=10" {
		t.Fatalf("unexpected key encoding: %q", key)
	}
}
