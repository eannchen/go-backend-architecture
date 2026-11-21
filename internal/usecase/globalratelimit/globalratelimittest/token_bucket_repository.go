package globalratelimittest

import (
	"context"
	"time"

	globalratelimit "github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
)

// TokenBucketRepository is a configurable double for its parent package's TokenBucketRepository contract.
type TokenBucketRepository struct {
	AllowFunc  func(ctx context.Context, key string, capacity int, refill time.Duration) (globalratelimit.StoreDecision, error)
	AllowCalls []TokenBucketRepositoryAllowCall
}

// TokenBucketRepositoryAllowCall records one TokenBucketRepository.Allow invocation.
type TokenBucketRepositoryAllowCall struct {
	Context  context.Context
	Key      string
	Capacity int
	Refill   time.Duration
}

func (d *TokenBucketRepository) Allow(ctx context.Context, key string, capacity int, refill time.Duration) (globalratelimit.StoreDecision, error) {
	d.AllowCalls = append(d.AllowCalls, TokenBucketRepositoryAllowCall{Context: ctx, Key: key, Capacity: capacity, Refill: refill})
	fn := d.AllowFunc
	if fn == nil {
		panic("unexpected TokenBucketRepository.Allow call")
	}
	return fn(ctx, key, capacity, refill)
}
