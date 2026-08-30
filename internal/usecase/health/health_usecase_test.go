package health

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/repository/cache/cachetest"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	"github.com/eannchen/go-backend-architecture/internal/repository/db/dbtest"
	"github.com/eannchen/go-backend-architecture/internal/repository/kvstore/kvstoretest"
)

func TestCheckReadySuccess(t *testing.T) {
	db := &dbtest.DBHealthRepository{
		PingFunc: func(context.Context) error { return nil },
		GetServerStatusFunc: func(context.Context) (repodb.DBServerStatus, error) {
			return repodb.DBServerStatus{
				DatabaseName:  "app",
				InRecovery:    false,
				UptimeSeconds: 123,
			}, nil
		},
		CheckVectorExtensionFunc: func(context.Context) error { return nil },
	}
	cache := &cachetest.CacheHealthStore{PingFunc: func(context.Context) error { return nil }}
	kv := &kvstoretest.KVHealthStore{PingFunc: func(context.Context) error { return nil }}
	uc := New(nil, nil, db, cache, kv)

	got, err := uc.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if got.Database.Status != "up" || got.Vector.Status != "up" || got.Cache.Status != "up" || got.KVStore.Status != "up" {
		t.Fatalf("unexpected health result: %+v", got)
	}
	if got.Database.Name != "app" || got.Database.UptimeSeconds != 123 {
		t.Fatalf("unexpected database payload: %+v", got.Database)
	}
	if db.PingCalls != 1 || db.GetServerStatusCalls != 1 || db.CheckVectorExtensionCalls != 1 || cache.PingCalls != 1 || kv.PingCalls != 1 {
		t.Fatalf("unexpected dependency call counts: dbPing=%d dbStatus=%d vector=%d cache=%d kv=%d", db.PingCalls, db.GetServerStatusCalls, db.CheckVectorExtensionCalls, cache.PingCalls, kv.PingCalls)
	}
}

func TestCheckLiveSkipsDependencies(t *testing.T) {
	db := &dbtest.DBHealthRepository{}
	cache := &cachetest.CacheHealthStore{}
	kv := &kvstoretest.KVHealthStore{}
	uc := New(nil, nil, db, cache, kv)

	got, err := uc.Check(context.Background(), CheckModeLive)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Database.Status != "skipped" || got.Vector.Status != "skipped" || got.Cache.Status != "skipped" || got.KVStore.Status != "skipped" {
		t.Fatalf("unexpected live result: %+v", got)
	}
	if db.PingCalls != 0 || db.GetServerStatusCalls != 0 || db.CheckVectorExtensionCalls != 0 || cache.PingCalls != 0 || kv.PingCalls != 0 {
		t.Fatalf("dependencies should not be called in live mode")
	}
}

func TestCheckInvalidMode(t *testing.T) {
	uc := New(nil, nil, &dbtest.DBHealthRepository{}, &cachetest.CacheHealthStore{}, &kvstoretest.KVHealthStore{})

	_, err := uc.Check(context.Background(), CheckMode("bad"))
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

func TestCheckCacheFailure(t *testing.T) {
	db := &dbtest.DBHealthRepository{
		PingFunc: func(context.Context) error { return nil },
		GetServerStatusFunc: func(context.Context) (repodb.DBServerStatus, error) {
			return repodb.DBServerStatus{DatabaseName: "app", UptimeSeconds: 10}, nil
		},
		CheckVectorExtensionFunc: func(context.Context) error { return nil },
	}
	cache := &cachetest.CacheHealthStore{PingFunc: func(context.Context) error { return errors.New("cache down") }}
	kv := &kvstoretest.KVHealthStore{}
	uc := New(nil, nil, db, cache, kv)

	got, err := uc.Check(context.Background(), CheckModeReady)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeUnavailable {
		t.Fatalf("expected unavailable app error, got %v", err)
	}
	if got.Database.Status != "up" || got.Vector.Status != "up" || got.Cache.Status != "down" || got.KVStore.Status != "skipped" {
		t.Fatalf("unexpected partial result on cache failure: %+v", got)
	}
	if kv.PingCalls != 0 {
		t.Fatalf("expected kv not called after cache failure, got %d", kv.PingCalls)
	}
}

func TestCheckVectorExtensionFailure(t *testing.T) {
	db := &dbtest.DBHealthRepository{
		PingFunc: func(context.Context) error { return nil },
		GetServerStatusFunc: func(context.Context) (repodb.DBServerStatus, error) {
			return repodb.DBServerStatus{DatabaseName: "app", UptimeSeconds: 10}, nil
		},
		CheckVectorExtensionFunc: func(context.Context) error {
			return errors.New("vector extension missing")
		},
	}
	cache := &cachetest.CacheHealthStore{}
	kv := &kvstoretest.KVHealthStore{}
	uc := New(nil, nil, db, cache, kv)

	got, err := uc.Check(context.Background(), CheckModeReady)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeUnavailable {
		t.Fatalf("expected unavailable app error, got %v", err)
	}
	if got.Database.Status != "up" || got.Vector.Status != "down" || got.Cache.Status != "skipped" || got.KVStore.Status != "skipped" {
		t.Fatalf("unexpected partial result on vector extension failure: %+v", got)
	}
	if cache.PingCalls != 0 || kv.PingCalls != 0 {
		t.Fatalf("expected cache and kv checks to be skipped, got cache=%d kv=%d", cache.PingCalls, kv.PingCalls)
	}
}

func TestNewWithNilTracerDoesNotPanic(t *testing.T) {
	uc := New(nil, nil, &dbtest.DBHealthRepository{}, &cachetest.CacheHealthStore{}, &kvstoretest.KVHealthStore{})

	if _, err := uc.Check(context.Background(), CheckModeLive); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
