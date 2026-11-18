package health

import "context"

// DBServerStatus is the database information required by readiness checks.
type DBServerStatus struct {
	DatabaseName  string
	InRecovery    bool
	UptimeSeconds int64
}

// DatabaseHealth supplies the database checks performed by this workflow.
type DatabaseHealth interface {
	Ping(context.Context) error
	GetServerStatus(context.Context) (DBServerStatus, error)
}

// Pinger checks availability. Cache and key-value services use the same operation.
type Pinger interface{ Ping(context.Context) error }
