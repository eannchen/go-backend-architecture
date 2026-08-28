//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/postgrestest"
	"github.com/eannchen/go-backend-architecture/internal/infra/redisconn/redistest"
)

var (
	httpTestPostgres *pgxpool.Pool
	httpTestRedis    *goredis.Client
)

// TestMain starts both real adapters once for this package. Individual tests
// still own their rows and keys because the containers are shared within the package.
func TestMain(m *testing.M) {
	os.Exit(runHTTPRealDependencyTests(m))
}

func runHTTPRealDependencyTests(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	testPostgres, err := postgrestest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start HTTP-test PostgreSQL dependency: %v\n", err)
		return 1
	}
	testRedis, err := redistest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start HTTP-test Redis dependency: %v\n", err)
		if closeErr := testPostgres.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "close HTTP-test PostgreSQL after Redis startup failure: %v\n", closeErr)
		}
		return 1
	}

	httpTestPostgres = testPostgres.Pool()
	httpTestRedis = testRedis.Client()
	exitCode := m.Run()
	exitCode = verifyHTTPTestDataCleanup(exitCode)

	if err := testRedis.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close HTTP-test Redis dependency: %v\n", err)
		exitCode = 1
	}
	if err := testPostgres.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close HTTP-test PostgreSQL dependency: %v\n", err)
		exitCode = 1
	}
	return exitCode
}

func verifyHTTPTestDataCleanup(exitCode int) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	remainingKeys, err := httpTestRedis.DBSize(ctx).Result()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify HTTP-test Redis cleanup: %v\n", err)
		return 1
	}
	if remainingKeys != 0 {
		fmt.Fprintf(os.Stderr, "verify HTTP-test Redis cleanup: %d key(s) remain\n", remainingKeys)
		return 1
	}

	var remainingRows int
	if err := httpTestPostgres.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM users) + (SELECT COUNT(*) FROM oauth_connections)
	`).Scan(&remainingRows); err != nil {
		fmt.Fprintf(os.Stderr, "verify HTTP-test PostgreSQL cleanup: %v\n", err)
		return 1
	}
	if remainingRows != 0 {
		fmt.Fprintf(os.Stderr, "verify HTTP-test PostgreSQL cleanup: %d application row(s) remain\n", remainingRows)
		return 1
	}
	return exitCode
}
