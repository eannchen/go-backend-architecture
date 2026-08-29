//go:build integration

package integration

import "testing"

func TestHealthFlow(t *testing.T) {
	fixture := newHealthFixture(t)

	t.Run("liveness does not require dependencies", func(t *testing.T) {
		got := fixture.getHealth(t, "live")
		if got.Database.Status != "skipped" || got.Cache.Status != "skipped" || got.Kvstore.Status != "skipped" || got.Vectorstore.Status != "skipped" {
			t.Fatalf("liveness response = %+v, want all dependencies skipped", got)
		}
	})

	t.Run("readiness reaches real PostgreSQL and Redis adapters", func(t *testing.T) {
		got := fixture.getHealth(t, "ready")
		if got.Database.Status != "up" || got.Database.Name != "integration_test" || got.Database.InRecovery || got.Database.UptimeSeconds < 0 {
			t.Fatalf("database readiness = %+v, want active integration database", got.Database)
		}
		if got.Cache.Status != "up" || got.Kvstore.Status != "up" || got.Vectorstore.Status != "up" {
			t.Fatalf("dependency readiness = %+v, want all dependencies up", got)
		}
	})
}
