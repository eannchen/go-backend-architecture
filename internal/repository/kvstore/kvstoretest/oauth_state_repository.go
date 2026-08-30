package kvstoretest

import (
	"context"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

// OAuthStateRepository is a reusable test double for repokvstore.OAuthStateRepository.
type OAuthStateRepository struct {
	StoreFunc    func(context.Context, string, repokvstore.OAuthStateData, time.Duration) error
	StoreCalls   int
	StoreState   string
	StoreData    repokvstore.OAuthStateData
	StoreTTL     time.Duration
	ConsumeFunc  func(context.Context, string) (repokvstore.OAuthStateData, bool, error)
	ConsumeCalls int
	ConsumeState string
}

func (r *OAuthStateRepository) Store(ctx context.Context, state string, data repokvstore.OAuthStateData, ttl time.Duration) error {
	r.StoreCalls++
	r.StoreState = state
	r.StoreData = data
	r.StoreTTL = ttl
	if r.StoreFunc == nil {
		panic("unexpected OAuthStateRepository.Store call")
	}
	return r.StoreFunc(ctx, state, data, ttl)
}

func (r *OAuthStateRepository) Consume(ctx context.Context, state string) (repokvstore.OAuthStateData, bool, error) {
	r.ConsumeCalls++
	r.ConsumeState = state
	if r.ConsumeFunc == nil {
		panic("unexpected OAuthStateRepository.Consume call")
	}
	return r.ConsumeFunc(ctx, state)
}

var _ repokvstore.OAuthStateRepository = (*OAuthStateRepository)(nil)
