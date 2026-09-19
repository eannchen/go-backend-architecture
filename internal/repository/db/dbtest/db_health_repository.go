package dbtest

import (
	"context"

	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

// DBHealthRepository is the canonical configurable double for repodb.DBHealthRepository.
type DBHealthRepository struct {
	PingFunc             func(context.Context) error
	PingCalls            int
	GetServerStatusFunc  func(context.Context) (repodb.DBServerStatus, error)
	GetServerStatusCalls int
}

func (r *DBHealthRepository) Ping(ctx context.Context) error {
	r.PingCalls++
	if r.PingFunc == nil {
		panic("unexpected DBHealthRepository.Ping call")
	}
	return r.PingFunc(ctx)
}

func (r *DBHealthRepository) GetServerStatus(ctx context.Context) (repodb.DBServerStatus, error) {
	r.GetServerStatusCalls++
	if r.GetServerStatusFunc == nil {
		panic("unexpected DBHealthRepository.GetServerStatus call")
	}
	return r.GetServerStatusFunc(ctx)
}

var _ repodb.DBHealthRepository = (*DBHealthRepository)(nil)
