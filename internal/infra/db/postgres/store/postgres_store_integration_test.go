//go:build integration

package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	// The production migration enables vector, so the test image must provide the
	// same extension. Pinning the tag keeps local and CI runs reproducible.
	postgresTestImage    = "pgvector/pgvector:0.8.6-pg18"
	postgresTestDatabase = "store_test"
	postgresTestUser     = "postgres"
	postgresTestPassword = "postgres"
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

	container, err := tcpostgres.Run(
		ctx,
		postgresTestImage,
		tcpostgres.WithDatabase(postgresTestDatabase),
		tcpostgres.WithUsername(postgresTestUser),
		tcpostgres.WithPassword(postgresTestPassword),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		// An explicitly requested integration suite must fail when its dependency
		// cannot start. Skipping here could make CI look green without testing SQL.
		fmt.Fprintf(os.Stderr, "start PostgreSQL integration container: %v\n", err)
		return 1
	}

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "get PostgreSQL integration connection string: %v\n", err)
		return terminatePostgresContainer(container, 1)
	}

	if err := migratePostgresTestDatabase(ctx, connectionString); err != nil {
		fmt.Fprintf(os.Stderr, "migrate PostgreSQL integration database: %v\n", err)
		return terminatePostgresContainer(container, 1)
	}

	postgresTestPool, err = pgxpool.New(ctx, connectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create PostgreSQL integration pool: %v\n", err)
		return terminatePostgresContainer(container, 1)
	}

	exitCode := m.Run()
	postgresTestPool.Close()
	return terminatePostgresContainer(container, exitCode)
}

func migratePostgresTestDatabase(ctx context.Context, connectionString string) error {
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer db.Close()

	// Resolve from this source file instead of the process working directory so
	// the suite behaves the same from an IDE, the repository root, and CI.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("locate integration harness source")
	}
	migrationDir := filepath.Join(filepath.Dir(filename), "..", "migrations")

	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		return fmt.Errorf("apply migrations from %s: %w", migrationDir, err)
	}
	return nil
}

func terminatePostgresContainer(container *tcpostgres.PostgresContainer, exitCode int) int {
	// Cleanup is explicit rather than relying only on Docker's resource reaper.
	// This makes the package lifecycle visible and avoids leaving test databases running.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := testcontainers.TerminateContainer(container, testcontainers.StopContext(ctx)); err != nil {
		fmt.Fprintf(os.Stderr, "terminate PostgreSQL integration container: %v\n", err)
		if exitCode == 0 {
			return 1
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
