package kvstoretest

import (
	"context"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

type TokenBucketRepository struct {
	AllowFunc  func(context.Context, string, int, time.Duration) (repokvstore.TokenBucketDecision, error)
	AllowCalls int
	Key        string
}

func (r *TokenBucketRepository) Allow(ctx context.Context, key string, capacity int, refill time.Duration) (repokvstore.TokenBucketDecision, error) {
	r.AllowCalls++
	r.Key = key
	if r.AllowFunc != nil {
		return r.AllowFunc(ctx, key, capacity, refill)
	}
	return repokvstore.TokenBucketDecision{}, nil
}

var _ repokvstore.TokenBucketRepository = (*TokenBucketRepository)(nil)
