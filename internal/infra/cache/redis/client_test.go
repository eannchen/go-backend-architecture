package redis

import "testing"

func TestSearchRuntimeValuesKey_Deterministic(t *testing.T) {
	keyA := searchRuntimeValuesKey("runtime.", 50)
	keyB := searchRuntimeValuesKey("runtime.", 50)
	if keyA != keyB {
		t.Fatalf("expected deterministic key, got %q and %q", keyA, keyB)
	}
}

func TestSearchRuntimeValuesKey_EncodesPrefix(t *testing.T) {
	key := searchRuntimeValuesKey("a b/c?", 10)
	if key != "runtime:search:prefix=a+b%2Fc%3F:limit=10" {
		t.Fatalf("unexpected key encoding: %q", key)
	}
}
