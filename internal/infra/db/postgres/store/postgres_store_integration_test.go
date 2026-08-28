//go:build integration

package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/postgrestest"
)

var postgresTestPool *pgxpool.Pool

// TestMain owns one container for this package. Sharing it keeps the suite fast,
// while cleanup owned by each test prevents rows from leaking into another test.
func TestMain(m *testing.M) {
	os.Exit(runPostgresStoreIntegrationTests(m))
}

func runPostgresStoreIntegrationTests(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	testPostgres, err := postgrestest.Start(ctx)
	if err != nil {
		// An explicitly requested integration suite must fail when its dependency
		// cannot start. Skipping here could make CI look green without testing SQL.
		fmt.Fprintf(os.Stderr, "start PostgreSQL store integration dependency: %v\n", err)
		return 1
	}
	postgresTestPool = testPostgres.Pool()

	exitCode := m.Run()
	if err := testPostgres.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close PostgreSQL store integration dependency: %v\n", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	return exitCode
}

func requirePostgresTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if postgresTestPool == nil {
		t.Fatal("PostgreSQL integration pool is not initialized")
	}
	return postgresTestPool
}
