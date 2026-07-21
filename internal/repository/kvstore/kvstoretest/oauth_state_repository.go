package kvstoretest

import (
	"context"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

// OAuthStateRepository is a reusable test double for repokvstore.OAuthStateRepository.
type OAuthStateRepository struct {
	StoreFunc     func(context.Context, string, repokvstore.OAuthStateData, time.Duration) error
	StoreCalls    int
	StoredState   string
	StoredData    repokvstore.OAuthStateData
	StoredTTL     time.Duration
	ConsumeFunc   func(context.Context, string) (repokvstore.OAuthStateData, bool, error)
	ConsumeCalls  int
	ConsumedState string
}

func (r *OAuthStateRepository) Store(ctx context.Context, state string, data repokvstore.OAuthStateData, ttl time.Duration) error {
	r.StoreCalls++
	r.StoredState = state
	r.StoredData = data
	r.StoredTTL = ttl
	if r.StoreFunc != nil {
		return r.StoreFunc(ctx, state, data, ttl)
	}
	return nil
}

func (r *OAuthStateRepository) Consume(ctx context.Context, state string) (repokvstore.OAuthStateData, bool, error) {
	r.ConsumeCalls++
	r.ConsumedState = state
	if r.ConsumeFunc != nil {
		return r.ConsumeFunc(ctx, state)
	}
	return repokvstore.OAuthStateData{}, false, nil
}

var _ repokvstore.OAuthStateRepository = (*OAuthStateRepository)(nil)
