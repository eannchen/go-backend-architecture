package globalratelimit

import (
	"context"
	"time"
)

// StoreDecision is the result of an atomic rate-limit check, regardless of algorithm.
type StoreDecision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// TokenBucketRepository admits burst-tolerant traffic at a sustained rate.
type TokenBucketRepository interface {
	Allow(ctx context.Context, key string, capacity int, refillInterval time.Duration) (StoreDecision, error)
}
