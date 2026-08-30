package cachetest

import (
	"context"

	repocache "github.com/eannchen/go-backend-architecture/internal/repository/cache"
)

// CacheHealthStore is the canonical configurable double for repocache.CacheHealthStore.
type CacheHealthStore struct {
	PingFunc  func(context.Context) error
	PingCalls int
}

func (s *CacheHealthStore) Ping(ctx context.Context) error {
	s.PingCalls++
	if s.PingFunc == nil {
		panic("unexpected CacheHealthStore.Ping call")
	}
	return s.PingFunc(ctx)
}

var _ repocache.CacheHealthStore = (*CacheHealthStore)(nil)
