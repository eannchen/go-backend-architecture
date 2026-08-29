package kvstoretest

import (
	"context"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

// KVHealthStore is the canonical configurable double for repokvstore.KVHealthStore.
type KVHealthStore struct {
	PingFunc  func(context.Context) error
	PingCalls int
}

func (s *KVHealthStore) Ping(ctx context.Context) error {
	s.PingCalls++
	if s.PingFunc == nil {
		panic("unexpected KVHealthStore.Ping call")
	}
	return s.PingFunc(ctx)
}

var _ repokvstore.KVHealthStore = (*KVHealthStore)(nil)
