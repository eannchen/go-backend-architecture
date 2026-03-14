package kvstore

import "context"

// KVHealthStore is used for key-value store readiness checks.
type KVHealthStore interface {
	Ping(ctx context.Context) error
}
