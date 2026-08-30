//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestDBHealthStore_Ping(t *testing.T) {
	store := NewDBHealthStore(requirePostgresTestPool(t), observability.NoopTracer{})

	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}
}

func TestDBHealthStore_GetServerStatus(t *testing.T) {
	store := NewDBHealthStore(requirePostgresTestPool(t), observability.NoopTracer{})

	status, err := store.GetServerStatus(context.Background())
	if err != nil {
		t.Fatalf("get PostgreSQL server status: %v", err)
	}
	if status.DatabaseName == "" {
		t.Fatal("expected database name, got an empty string")
	}
	if status.InRecovery {
		t.Fatal("expected the disposable PostgreSQL instance to be a primary server")
	}
	if status.UptimeSeconds < 0 {
		t.Fatalf("uptime seconds = %d, want a non-negative value", status.UptimeSeconds)
	}
}

func TestDBHealthStore_CheckVectorExtension(t *testing.T) {
	store := NewDBHealthStore(requirePostgresTestPool(t), observability.NoopTracer{})

	// Migrations install the vector extension, so this verifies both the health
	// adapter's query and the database state required by the application.
	if err := store.CheckVectorExtension(context.Background()); err != nil {
		t.Fatalf("check vector extension: %v", err)
	}
}
