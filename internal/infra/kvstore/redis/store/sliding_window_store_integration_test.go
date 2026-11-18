//go:build integration

package store

import (
	"strconv"
	"testing"
	"time"
)

// TestSlidingWindowStoreIntegration checks admissions, remaining quota, retry
// delays, and expiring Redis state for a single rolling-window limit.
func TestSlidingWindowStoreIntegration(t *testing.T) {
	const window = time.Minute
	store := NewSlidingWindowStore(redisTestClient)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	for _, limit := range []int{1, 2} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			key := "integration:" + suffix + ":" + strconv.Itoa(limit)
			redisKey := slidingWindowKeyPrefix + key
			t.Cleanup(func() { cleanupRedisKeys(t, redisKey) })
			for call := 0; call < limit; call++ {
				got, err := store.Allow(t.Context(), key, limit, window)
				if err != nil {
					t.Fatal(err)
				}
				if !got.Allowed || got.Remaining != limit-call-1 || got.RetryAfter != 0 {
					t.Fatalf("admission %d = %+v, want allowed with %d remaining", call+1, got, limit-call-1)
				}
			}
			got, err := store.Allow(t.Context(), key, limit, window)
			if err != nil {
				t.Fatal(err)
			}
			if got.Allowed || got.Remaining != 0 || got.RetryAfter <= 0 || got.RetryAfter > window {
				t.Fatalf("exhausted limit = %+v, want rejection with a retry delay within the window", got)
			}
			requirePositiveRedisTTL(t, redisKey, window)
		})
	}
}
