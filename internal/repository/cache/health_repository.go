package cache

import "context"

// CacheHealthStore is used for cache readiness checks.
type CacheHealthStore interface {
	Ping(ctx context.Context) error
}
