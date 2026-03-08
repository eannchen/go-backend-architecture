package repository

import (
	"context"
	"time"
)

// KVStore defines generic key-value operations for usecases.
// It is storage-agnostic and can be implemented by Redis or other KV engines.
type KVStore interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Delete(ctx context.Context, key string) error
}
