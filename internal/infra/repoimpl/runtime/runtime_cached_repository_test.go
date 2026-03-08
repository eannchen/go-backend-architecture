package runtimerepo

import (
	"context"
	"errors"
	"testing"

	"go-backend-architecture/internal/infra/logger"
	"go-backend-architecture/internal/infra/observability"
	"go-backend-architecture/internal/repository"
)

func TestRuntimeCachedRepository_SearchRuntimeValues_CacheHit(t *testing.T) {
	base := &fakeRuntimeRepository{
		searchResult: []repository.RuntimeKV{{Key: "k1", Value: "v1"}},
	}
	cache := &fakeRuntimeCache{
		getFound: true,
		getItems: []repository.RuntimeKV{{Key: "cached", Value: "value"}},
	}
	repo := NewRuntimeCachedRepository(testLogger{}, observability.NoopTracer{}, base, cache)

	items, err := repo.SearchRuntimeValues(context.Background(), "sys", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base.searchCalls != 0 {
		t.Fatalf("expected db search not called, got %d", base.searchCalls)
	}
	if len(items) != 1 || items[0].Key != "cached" {
		t.Fatalf("unexpected cache items: %#v", items)
	}
	if cache.setCalls != 0 {
		t.Fatalf("expected cache set not called on hit")
	}
}

func TestRuntimeCachedRepository_SearchRuntimeValues_CacheMissFallsBack(t *testing.T) {
	base := &fakeRuntimeRepository{
		searchResult: []repository.RuntimeKV{{Key: "db", Value: "value"}},
	}
	cache := &fakeRuntimeCache{}
	repo := NewRuntimeCachedRepository(testLogger{}, observability.NoopTracer{}, base, cache)

	items, err := repo.SearchRuntimeValues(context.Background(), "sys", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base.searchCalls != 1 {
		t.Fatalf("expected db search called once, got %d", base.searchCalls)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected cache set called once, got %d", cache.setCalls)
	}
	if len(items) != 1 || items[0].Key != "db" {
		t.Fatalf("unexpected db items: %#v", items)
	}
}

func TestRuntimeCachedRepository_SearchRuntimeValues_CacheReadErrorFallsBack(t *testing.T) {
	base := &fakeRuntimeRepository{
		searchResult: []repository.RuntimeKV{{Key: "db", Value: "value"}},
	}
	cache := &fakeRuntimeCache{
		getErr: errors.New("cache down"),
	}
	repo := NewRuntimeCachedRepository(testLogger{}, observability.NoopTracer{}, base, cache)

	_, err := repo.SearchRuntimeValues(context.Background(), "sys", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base.searchCalls != 1 {
		t.Fatalf("expected db search called once, got %d", base.searchCalls)
	}
}

type fakeRuntimeRepository struct {
	searchResult []repository.RuntimeKV
	searchErr    error
	searchCalls  int
}

func (f *fakeRuntimeRepository) Ping(ctx context.Context) error { return nil }

func (f *fakeRuntimeRepository) GetServerStatus(ctx context.Context) (repository.DBServerStatus, error) {
	return repository.DBServerStatus{}, nil
}

func (f *fakeRuntimeRepository) GetRuntimeValue(ctx context.Context, key string) (repository.RuntimeKV, error) {
	return repository.RuntimeKV{}, nil
}

func (f *fakeRuntimeRepository) SearchRuntimeValues(ctx context.Context, prefix string, limit uint64) ([]repository.RuntimeKV, error) {
	f.searchCalls++
	return f.searchResult, f.searchErr
}

type fakeRuntimeCache struct {
	getItems []repository.RuntimeKV
	getFound bool
	getErr   error
	setErr   error
	setCalls int
}

func (f *fakeRuntimeCache) GetSearchRuntimeValues(ctx context.Context, prefix string, limit uint64) ([]repository.RuntimeKV, bool, error) {
	return f.getItems, f.getFound, f.getErr
}

func (f *fakeRuntimeCache) SetSearchRuntimeValues(ctx context.Context, prefix string, limit uint64, items []repository.RuntimeKV) error {
	f.setCalls++
	return f.setErr
}

type testLogger struct{}

func (testLogger) Debug(ctx context.Context, message string, fields ...logger.Fields) {}
func (testLogger) Info(ctx context.Context, message string, fields ...logger.Fields)  {}
func (testLogger) Warn(ctx context.Context, message string, fields ...logger.Fields)  {}
func (testLogger) Error(ctx context.Context, message string, err error, fields ...logger.Fields) {
}
func (testLogger) SetLogSink(sink logger.LogSinkFunc) {}
func (testLogger) SetContextFieldsProvider(provider logger.ContextFieldsProviderFunc) {
}
func (testLogger) Sync() error { return nil }
