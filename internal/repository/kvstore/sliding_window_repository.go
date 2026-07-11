package kvstore

import (
	"context"
	"time"
)

type SlidingWindowDecision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type SlidingWindowTier struct {
	Key    string
	Limit  int
	Window time.Duration
}

type SlidingWindowTieredDecision struct {
	Allowed      bool
	RejectedTier string
	RetryAfter   time.Duration
}

// SlidingWindowRepository admits a strict number of events in a rolling window.
type SlidingWindowRepository interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (SlidingWindowDecision, error)
	AllowTiered(ctx context.Context, tiers []SlidingWindowTier) (SlidingWindowTieredDecision, error)
}
