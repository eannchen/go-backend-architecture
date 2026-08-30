//go:build integration

package store

import (
	"os"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/infra/redisconn/redistest"
)

var redisTestClient *goredis.Client

// TestMain owns one Redis container for this package. Tests share the service
// for speed, but each test remains responsible for deleting the keys it creates.
func TestMain(m *testing.M) {
	os.Exit(redistest.RunPackage(m, "cache-store", func(client *goredis.Client) {
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
