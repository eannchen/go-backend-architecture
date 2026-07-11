package kvstore

import (
	"context"
	"time"
)

type TokenBucketDecision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// TokenBucketRepository admits burst-tolerant traffic at a sustained rate.
type TokenBucketRepository interface {
	Allow(ctx context.Context, key string, capacity int, refillInterval time.Duration) (TokenBucketDecision, error)
}
