package health_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/usecase/health"
	healthtest "github.com/eannchen/go-backend-architecture/internal/usecase/health/healthtest"
)

// TestCheckReadySuccess checks readiness combines successful dependency checks into a healthy result.
func TestCheckReadySuccess(t *testing.T) {
	db := &healthtest.DatabaseHealth{
		PingFunc: func(context.Context) error { return nil },
		GetServerStatusFunc: func(context.Context) (health.DBServerStatus, error) {
			return health.DBServerStatus{
				DatabaseName:  "app",
				InRecovery:    false,
				UptimeSeconds: 123,
			}, nil
		},
	}
	cache := &healthtest.Pinger{PingFunc: func(context.Context) error { return nil }}
	kv := &healthtest.Pinger{PingFunc: func(context.Context) error { return nil }}
	uc := health.New(observability.NoopTracer{}, observability.NoopMeter{}, db, cache, kv)

	got, err := uc.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if got.Database.Status != "up" || got.Cache.Status != "up" || got.KVStore.Status != "up" {
		t.Fatalf("unexpected health result: %+v", got)
	}
	if got.Database.Name != "app" || got.Database.UptimeSeconds != 123 {
		t.Fatalf("unexpected database payload: %+v", got.Database)
	}
	if len(db.PingCalls) != 1 || len(db.GetServerStatusCalls) != 1 || len(cache.PingCalls) != 1 || len(kv.PingCalls) != 1 {
		t.Fatalf("unexpected dependency call counts: dbPing=%d dbStatus=%d cache=%d kv=%d", len(db.PingCalls), len(db.GetServerStatusCalls), len(cache.PingCalls), len(kv.PingCalls))
	}
}

// TestCheckLiveSkipsDependencies checks liveness stays available without probing external dependencies.
func TestCheckLiveSkipsDependencies(t *testing.T) {
	db := &healthtest.DatabaseHealth{}
	cache := &healthtest.Pinger{}
	kv := &healthtest.Pinger{}
	uc := health.New(observability.NoopTracer{}, observability.NoopMeter{}, db, cache, kv)

	got, err := uc.Check(context.Background(), health.CheckModeLive)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Database.Status != "skipped" || got.Cache.Status != "skipped" || got.KVStore.Status != "skipped" {
		t.Fatalf("unexpected live result: %+v", got)
	}
	if len(db.PingCalls) != 0 || len(db.GetServerStatusCalls) != 0 || len(cache.PingCalls) != 0 || len(kv.PingCalls) != 0 {
		t.Fatalf("dependencies should not be called in live mode")
	}
}

// TestCheckInvalidMode checks unsupported health modes fail before probing dependencies.
func TestCheckInvalidMode(t *testing.T) {
	uc := health.New(observability.NoopTracer{}, observability.NoopMeter{}, &healthtest.DatabaseHealth{}, &healthtest.Pinger{}, &healthtest.Pinger{})

	_, err := uc.Check(context.Background(), health.CheckMode("bad"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected apperr, got %T", err)
	}
	if appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("unexpected code: %s", appErr.Code)
	}
}

// TestCheckCacheFailure checks cache failure makes readiness unavailable with useful detail.
func TestCheckCacheFailure(t *testing.T) {
	db := &healthtest.DatabaseHealth{
		PingFunc: func(context.Context) error { return nil },
		GetServerStatusFunc: func(context.Context) (health.DBServerStatus, error) {
			return health.DBServerStatus{DatabaseName: "app", UptimeSeconds: 10}, nil
		},
	}
	cache := &healthtest.Pinger{PingFunc: func(context.Context) error { return errors.New("cache down") }}
	kv := &healthtest.Pinger{}
	uc := health.New(observability.NoopTracer{}, observability.NoopMeter{}, db, cache, kv)

	got, err := uc.Check(context.Background(), health.CheckModeReady)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeUnavailable {
		t.Fatalf("expected unavailable app error, got %v", err)
	}
	if got.Database.Status != "up" || got.Cache.Status != "down" || got.KVStore.Status != "skipped" {
		t.Fatalf("unexpected partial result on cache failure: %+v", got)
	}
	if len(kv.PingCalls) != 0 {
		t.Fatalf("expected kv not called after cache failure, got %d", len(kv.PingCalls))
	}
}
