package health

import (
	"context"
	"errors"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/repository"
)

type stubDBHealthRepo struct {
	pingErr               error
	status                repository.DBServerStatus
	statusErr             error
	vectorExtensionErr    error
	pingCalls             int
	statusCalls           int
	vectorExtensionChecks int
}

func (s *stubDBHealthRepo) Ping(context.Context) error {
	s.pingCalls++
	return s.pingErr
}

func (s *stubDBHealthRepo) GetServerStatus(context.Context) (repository.DBServerStatus, error) {
	s.statusCalls++
	if s.statusErr != nil {
		return repository.DBServerStatus{}, s.statusErr
	}
	return s.status, nil
}

func (s *stubDBHealthRepo) CheckVectorExtension(context.Context) error {
	s.vectorExtensionChecks++
	return s.vectorExtensionErr
}

type stubHealthStore struct {
	err   error
	calls int
}

func (s *stubHealthStore) Ping(context.Context) error {
	s.calls++
	return s.err
}

func TestCheckReadySuccess(t *testing.T) {
	db := &stubDBHealthRepo{
		status: repository.DBServerStatus{
			DatabaseName:  "app",
			InRecovery:    false,
			UptimeSeconds: 123,
		},
	}
	cache := &stubHealthStore{}
	kv := &stubHealthStore{}
	uc := New(observability.NoopTracer{}, observability.NoopMeter{}, db, cache, kv)

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
	if db.pingCalls != 1 || db.statusCalls != 1 || db.vectorExtensionChecks != 1 || cache.calls != 1 || kv.calls != 1 {
		t.Fatalf("unexpected dependency call counts: dbPing=%d dbStatus=%d vector=%d cache=%d kv=%d", db.pingCalls, db.statusCalls, db.vectorExtensionChecks, cache.calls, kv.calls)
	}
}

func TestCheckLiveSkipsDependencies(t *testing.T) {
	db := &stubDBHealthRepo{}
	cache := &stubHealthStore{}
	kv := &stubHealthStore{}
	uc := New(observability.NoopTracer{}, observability.NoopMeter{}, db, cache, kv)

	got, err := uc.Check(context.Background(), CheckModeLive)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Database.Status != "skipped" || got.Vector.Status != "skipped" || got.Cache.Status != "skipped" || got.KVStore.Status != "skipped" {
		t.Fatalf("unexpected live result: %+v", got)
	}
	if db.pingCalls != 0 || db.statusCalls != 0 || db.vectorExtensionChecks != 0 || cache.calls != 0 || kv.calls != 0 {
		t.Fatalf("dependencies should not be called in live mode")
	}
}

func TestCheckInvalidMode(t *testing.T) {
	uc := New(observability.NoopTracer{}, observability.NoopMeter{}, &stubDBHealthRepo{}, &stubHealthStore{}, &stubHealthStore{})

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
	db := &stubDBHealthRepo{
		status: repository.DBServerStatus{DatabaseName: "app", UptimeSeconds: 10},
	}
	cache := &stubHealthStore{err: errors.New("cache down")}
	kv := &stubHealthStore{}
	uc := New(observability.NoopTracer{}, observability.NoopMeter{}, db, cache, kv)

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
	if kv.calls != 0 {
		t.Fatalf("expected kv not called after cache failure, got %d", kv.calls)
	}
}

func TestCheckVectorExtensionFailure(t *testing.T) {
	db := &stubDBHealthRepo{
		status:             repository.DBServerStatus{DatabaseName: "app", UptimeSeconds: 10},
		vectorExtensionErr: errors.New("vector extension missing"),
	}
	cache := &stubHealthStore{}
	kv := &stubHealthStore{}
	uc := New(observability.NoopTracer{}, observability.NoopMeter{}, db, cache, kv)

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
	if cache.calls != 0 || kv.calls != 0 {
		t.Fatalf("expected cache and kv checks to be skipped, got cache=%d kv=%d", cache.calls, kv.calls)
	}
}

func TestNewWithNilTracerDoesNotPanic(t *testing.T) {
	uc := New(nil, observability.NoopMeter{}, &stubDBHealthRepo{}, &stubHealthStore{}, &stubHealthStore{})

	if _, err := uc.Check(context.Background(), CheckModeLive); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
