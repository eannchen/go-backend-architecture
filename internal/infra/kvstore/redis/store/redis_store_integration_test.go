//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/infra/redisconn/redistest"
)

var redisTestClient *goredis.Client

// TestMain owns one Redis container for this package. Tests share the service
// for speed, but each test remains responsible for deleting the keys it creates.
func TestMain(m *testing.M) {
	os.Exit(redistest.RunPackage(m, "key-value-store", func(client *goredis.Client) {
		redisTestClient = client
	}))
}

func requireRedisTestClient(t *testing.T) *goredis.Client {
	t.Helper()

	if redisTestClient == nil {
		t.Fatal("Redis integration client is not initialized")
	}
	return redisTestClient
}

func requirePositiveRedisTTL(t *testing.T, key string, maximum time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ttl, err := redisTestClient.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read Redis TTL for %q: %v", key, err)
	}
	if ttl <= 0 || ttl > maximum {
		t.Fatalf("Redis TTL for %q = %v, want within (0, %v]", key, ttl, maximum)
	}
}

func cleanupRedisKeys(t *testing.T, keys ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := redisTestClient.Del(ctx, keys...).Err(); err != nil {
		t.Errorf("cleanup Redis keys %v: %v", keys, err)
	}
}
