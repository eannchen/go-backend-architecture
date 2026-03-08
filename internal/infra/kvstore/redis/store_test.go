package redis

import (
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestPrefixedKey(t *testing.T) {
	store := NewStore(&goredis.Client{}, "session")
	if got := store.prefixedKey("abc"); got != "session:abc" {
		t.Fatalf("unexpected prefixed key: %q", got)
	}
}

func TestPrefixedKey_EmptyPrefix(t *testing.T) {
	store := NewStore(&goredis.Client{}, "")
	if got := store.prefixedKey("abc"); got != "abc" {
		t.Fatalf("unexpected prefixed key without prefix: %q", got)
	}
}
