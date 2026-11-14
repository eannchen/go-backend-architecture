package kafkaworker

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// TestAppBuildsEveryReaderAndStops verifies the real composition path creates
// both workflows under either policy and releases clients before telemetry.
func TestAppBuildsEveryReaderAndStops(t *testing.T) {
	for _, policy := range []string{"stop-all", "independent"} {
		t.Run(policy, func(t *testing.T) {
			t.Setenv("OTEL_EXPORT_ENABLED", "false")
			t.Setenv("KAFKA_BROKERS", "127.0.0.1:1")
			t.Setenv("KAFKA_NOTIFICATION_TOPIC", "mail")
			t.Setenv("KAFKA_ACTIVITY_TOPIC", "actions")
			t.Setenv("KAFKA_WORKER_FAILURE_POLICY", policy)
			app, err := New(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			// Terminal log-sink behavior is outside this lifecycle test.
			app.Telemetry.Logger = logger.NoopLogger{}
			started := false
			t.Cleanup(func() {
				cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if started {
					if err := app.Shutdown(cleanup); err != nil {
						t.Error(err)
					}
					return
				}
				for _, worker := range app.workers {
					if err := worker.Shutdown(cleanup); err != nil {
						t.Error(err)
					}
				}
				if err := app.Telemetry.Shutdown(cleanup); err != nil {
					t.Error(err)
				}
			})
			if len(app.workers) != 2 || app.workers[0].Name != "mail" || app.workers[1].Name != "actions" {
				t.Fatal("application did not build both configured workflows")
			}
			// Cancellation before polling exercises startup/shutdown without requiring
			// a live broker. Processing and failure coordination have their own tests.
			run, cancel := context.WithCancel(t.Context())
			cancel()
			started = true
			if err := app.Start(run); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestAppReportsLaterReaderInitFailure exercises rollback after the first
// reader has been built, preserving the cause from the second reader's TLS load.
func TestAppReportsLaterReaderInitFailure(t *testing.T) {
	t.Setenv("OTEL_EXPORT_ENABLED", "false")
	t.Setenv("KAFKA_BROKERS", "127.0.0.1:1")
	t.Setenv("KAFKA_NOTIFICATION_TOPIC", "mail")
	t.Setenv("KAFKA_ACTIVITY_TOPIC", "actions")
	t.Setenv("KAFKA_NOTIFICATION_TLS_ENABLED", "false")
	t.Setenv("KAFKA_ACTIVITY_TLS_ENABLED", "true")
	missing := filepath.Join(t.TempDir(), "missing-ca.pem")
	t.Setenv("KAFKA_ACTIVITY_TLS_SERVER_CA_FILE", missing)
	app, err := New(t.Context())
	if app != nil || err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatal("second reader TLS failure was lost")
	}
}

// TestAppFailurePolicyWiring verifies the selected policy reaches the shared
// coordinator through Start, while Shutdown waits for every worker to finish.
func TestAppFailurePolicyWiring(t *testing.T) {
	for _, independent := range []bool{false, true} {
		name := "stop-all"
		if independent {
			name = "independent"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORT_ENABLED", "false")
			t.Setenv("KAFKA_WORKER_FAILURE_POLICY", name)
			cfg, err := config.LoadKafkaWorker()
			if err != nil {
				t.Fatal(err)
			}
			telemetry, err := appruntime.NewTelemetry(t.Context(), cfg.RuntimeConfig)
			if err != nil {
				t.Fatal(err)
			}
			telemetry.Logger = logger.NoopLogger{}
			ready := make(chan context.Context, 1)
			failedClosed := make(chan struct{})
			app := &App{Telemetry: telemetry, Config: cfg, done: make(chan struct{}), workers: []appruntime.Worker{
				{Name: "failed", Run: func(ctx context.Context) error { return errors.New("processing failed") },
					Shutdown: func(context.Context) error { close(failedClosed); return nil }},
				{Name: "healthy", Run: func(ctx context.Context) error { ready <- ctx; <-ctx.Done(); return nil },
					Shutdown: func(context.Context) error { return nil }},
			}}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- app.Start(ctx) }()
			var running context.Context
			select {
			case running = <-ready:
			case <-ctx.Done():
				t.Fatal("healthy worker did not start")
			}
			select {
			case <-failedClosed:
			case <-ctx.Done():
				t.Fatal("failed reader was not closed")
			}
			if independent {
				if running.Err() != nil {
					t.Fatal("independent worker was canceled")
				}
				cancel()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("failure was lost")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("workers did not stop")
			}
			cleanup, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			if err := app.Shutdown(cleanup); err != nil {
				t.Fatal(err)
			}
		})
	}
}
