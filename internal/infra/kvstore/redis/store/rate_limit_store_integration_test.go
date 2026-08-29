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
	const refillInterval = time.Minute

	client := requireRedisTestClient(t)
	store := NewTokenBucketStore(client)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	tests := []struct {
		name        string
		capacity    int
		wantAllowed []bool
	}{
		{name: "one request capacity", capacity: 1, wantAllowed: []bool{true, false}},
		{name: "two request capacity", capacity: 2, wantAllowed: []bool{true, true, false}},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "integration:" + suffix + ":" + strconv.Itoa(index)
			redisKey := tokenBucketKeyPrefix + key
			t.Cleanup(func() { cleanupRedisKeys(t, redisKey) })

			for call, wantAllowed := range tt.wantAllowed {
				got, err := store.Allow(context.Background(), key, tt.capacity, refillInterval)
				if err != nil {
					t.Fatalf("allow call %d: %v", call+1, err)
				}
				if got.Allowed != wantAllowed {
					t.Fatalf("allow call %d = %v, want %v", call+1, got.Allowed, wantAllowed)
				}
				if !got.Allowed && got.RetryAfter <= 0 {
					t.Fatalf("rejected call %d retry after = %v, want positive duration", call+1, got.RetryAfter)
				}
			}

			requirePositiveRedisTTL(t, redisKey, time.Duration(tt.capacity)*refillInterval)
		})
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
