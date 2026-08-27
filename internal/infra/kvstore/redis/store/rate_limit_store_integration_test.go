//go:build integration

package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

func TestTokenBucketStoreIntegration(t *testing.T) {
	client := requireRedisTestClient(t)
	key := "integration:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	redisKey := tokenBucketKeyPrefix + key
	t.Cleanup(func() { cleanupRedisKeys(t, redisKey) })

	store := NewTokenBucketStore(client)
	if got, err := store.Allow(context.Background(), key, 1, time.Minute); err != nil || !got.Allowed {
		t.Fatalf("first allow = %+v, %v", got, err)
	}
	requirePositiveRedisTTL(t, redisKey, time.Minute)
	if got, err := store.Allow(context.Background(), key, 1, time.Minute); err != nil || got.Allowed || got.RetryAfter <= 0 {
		t.Fatalf("second allow = %+v, %v", got, err)
	}
}

func TestSlidingWindowStoreTieredIntegration(t *testing.T) {
	client := requireRedisTestClient(t)
	key := "integration:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	redisKey := slidingWindowKeyPrefix + key
	t.Cleanup(func() { cleanupRedisKeys(t, redisKey) })

	store := NewSlidingWindowStore(client)
	tiers := []repokvstore.SlidingWindowTier{{Key: key, Limit: 1, Window: time.Minute}}
	if got, err := store.AllowTiered(context.Background(), tiers); err != nil || !got.Allowed {
		t.Fatalf("first tiered allow = %+v, %v", got, err)
	}
	requirePositiveRedisTTL(t, redisKey, time.Minute)
	if got, err := store.AllowTiered(context.Background(), tiers); err != nil || got.Allowed || got.RejectedTier != key {
		t.Fatalf("second tiered allow = %+v, %v", got, err)
	}
}
