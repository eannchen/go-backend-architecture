package repository

import "context"

// RuntimeKV is infrastructure-neutral runtime metadata.
type RuntimeKV struct {
	Key   string
	Value string
}

// RuntimeRepository defines persistence operations used by usecases.
//
// The contract is storage-agnostic; infrastructure chooses implementation details.
type RuntimeRepository interface {
	Ping(ctx context.Context) error
	GetRuntimeValue(ctx context.Context, key string) (RuntimeKV, error)
	SearchRuntimeValues(ctx context.Context, prefix string, limit uint64) ([]RuntimeKV, error)
}
