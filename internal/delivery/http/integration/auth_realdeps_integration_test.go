//go:build integration

package integration

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestAuthRealDepsConnectivity(t *testing.T) {
	dbURL := os.Getenv("DB_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dbURL == "" || redisAddr == "" {
		t.Skip("set DB_URL and REDIS_ADDR to run real dependency integration checks")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Skipf("skip: unable to initialize postgres driver: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skip: postgres unavailable: %v", err)
	}

	redisDB := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			t.Skipf("skip: invalid REDIS_DB=%q: %v", v, err)
		}
		redisDB = parsed
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       redisDB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("skip: redis unavailable: %v", err)
	}
}
