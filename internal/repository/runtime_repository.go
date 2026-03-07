package repository

import "context"

// RuntimeKV is infrastructure-neutral runtime metadata.
type RuntimeKV struct {
	Key   string
	Value string
}

type DBServerStatus struct {
	DatabaseName  string
	InRecovery    bool
	UptimeSeconds int64
}

// RuntimeRepository defines persistence operations used by usecases.
//
// The contract is storage-agnostic; infrastructure chooses implementation details.
type RuntimeRepository interface {
	Ping(ctx context.Context) error
	GetServerStatus(ctx context.Context) (DBServerStatus, error)
	GetRuntimeValue(ctx context.Context, key string) (RuntimeKV, error)
	SearchRuntimeValues(ctx context.Context, prefix string, limit uint64) ([]RuntimeKV, error)
}
