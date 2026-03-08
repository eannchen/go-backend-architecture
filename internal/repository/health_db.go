package repository

import "context"

type DBServerStatus struct {
	DatabaseName  string
	InRecovery    bool
	UptimeSeconds int64
}

// DBHealthRepository is used for database readiness checks.
type DBHealthRepository interface {
	Ping(ctx context.Context) error
	GetServerStatus(ctx context.Context) (DBServerStatus, error)
}
