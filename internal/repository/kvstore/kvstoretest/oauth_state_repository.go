package kvstoretest

import (
	"context"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

// OAuthStateRepository is a reusable test double for repokvstore.OAuthStateRepository.
type OAuthStateRepository struct {
	StoreFunc     func(context.Context, string, time.Duration) error
	StoreCalls    int
	StoredState   string
	StoredTTL     time.Duration
	ConsumeFunc   func(context.Context, string) (bool, error)
	ConsumeCalls  int
	ConsumedState string
}

func (r *OAuthStateRepository) Store(ctx context.Context, state string, ttl time.Duration) error {
	r.StoreCalls++
	r.StoredState = state
	r.StoredTTL = ttl
	if r.StoreFunc != nil {
		return r.StoreFunc(ctx, state, ttl)
	}
	return nil
}

func (r *OAuthStateRepository) Consume(ctx context.Context, state string) (bool, error) {
	r.ConsumeCalls++
	r.ConsumedState = state
	if r.ConsumeFunc != nil {
		return r.ConsumeFunc(ctx, state)
	}
	return false, nil
}

var _ repokvstore.OAuthStateRepository = (*OAuthStateRepository)(nil)
