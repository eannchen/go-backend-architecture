package kvstoretest

import (
	"context"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

type TokenBucketRepository struct {
	AllowFunc     func(context.Context, string, int, time.Duration) (repokvstore.TokenBucketDecision, error)
	AllowCalls    int
	AllowKey      string
	AllowCapacity int
	AllowRefill   time.Duration
}

func (r *TokenBucketRepository) Allow(ctx context.Context, key string, capacity int, refill time.Duration) (repokvstore.TokenBucketDecision, error) {
	r.AllowCalls++
	r.AllowKey = key
	r.AllowCapacity = capacity
	r.AllowRefill = refill
	if r.AllowFunc == nil {
		panic("unexpected TokenBucketRepository.Allow call")
	}
	return r.AllowFunc(ctx, key, capacity, refill)
}

var _ repokvstore.TokenBucketRepository = (*TokenBucketRepository)(nil)
