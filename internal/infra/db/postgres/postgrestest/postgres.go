//go:build integration

// Package postgrestest owns disposable PostgreSQL infrastructure for integration tests.
package postgrestest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
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
	postgresTestDatabase = "integration_test"
	postgresTestUser     = "postgres"
	postgresTestPassword = "postgres"
)

// Instance is one migrated PostgreSQL container owned by a test package.
type Instance struct {
	container *tcpostgres.PostgresContainer
	pool      *pgxpool.Pool
}

// Start creates a fresh PostgreSQL instance and applies all production migrations.
func Start(ctx context.Context) (*Instance, error) {
	container, err := tcpostgres.Run(
		ctx,
		postgresTestImage,
		tcpostgres.WithDatabase(postgresTestDatabase),
		tcpostgres.WithUsername(postgresTestUser),
		tcpostgres.WithPassword(postgresTestPassword),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start PostgreSQL container: %w", err)
	}

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("get PostgreSQL connection string: %w", err),
			testcontainers.TerminateContainer(container),
		)
	}
	if err := migrate(ctx, connectionString); err != nil {
		return nil, errors.Join(err, testcontainers.TerminateContainer(container))
	}

	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("create PostgreSQL pool: %w", err),
			testcontainers.TerminateContainer(container),
		)
	}

	return &Instance{
		container: container,
		pool:      pool,
	}, nil
}

// Pool returns the package-shared pool; individual tests must not close it.
func (i *Instance) Pool() *pgxpool.Pool {
	return i.pool
}

// Close closes the pool and explicitly removes the package container.
func (i *Instance) Close() error {
	i.pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return testcontainers.TerminateContainer(i.container, testcontainers.StopContext(ctx))
}

func migrate(ctx context.Context, connectionString string) error {
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer db.Close()

	// Resolve migrations relative to this source file so the same code works from
	// the repository root, an IDE package runner, and a CI checkout.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("locate PostgreSQL test helper source")
	}
	migrationDir := filepath.Join(filepath.Dir(filename), "..", "migrations")
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		return fmt.Errorf("apply migrations from %s: %w", migrationDir, err)
	}
	return nil
}
