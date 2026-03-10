package store

import (
	"context"

	dbsqlc "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/sqlc/gen"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/repository"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

type DBHealthStore struct {
	queries *dbsqlc.Queries
	tracer  observability.Tracer
}

func NewDBHealthStore(db dbsqlc.DBTX, tracer observability.Tracer) *DBHealthStore {
	return &DBHealthStore{
		queries: dbsqlc.New(db),
		tracer:  tracer,
	}
}

func (r *DBHealthStore) Ping(ctx context.Context) (err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "db_health_store.ping",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "ping",
		),
	)
	defer func() {
		span.Finish(err)
	}()

	_, err = r.queries.Ping(ctx)
	if err != nil {
		return apperr.Wrap(err, apperr.CodeUnavailable, "database ping failed")
	}
	return nil
}

func (r *DBHealthStore) GetServerStatus(ctx context.Context) (status repository.DBServerStatus, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "db_health_store.get_server_status",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "select",
			"db.statement.type", "server_status",
		),
	)
	defer func() {
		span.Finish(err)
	}()

	row, err := r.queries.GetServerStatus(ctx)
	if err != nil {
		return repository.DBServerStatus{}, apperr.Wrap(err, apperr.CodeUnavailable, "database server status query failed")
	}

	status = repository.DBServerStatus{
		DatabaseName:  row.DatabaseName,
		InRecovery:    row.InRecovery,
		UptimeSeconds: row.UptimeSeconds,
	}
	return status, nil
}

var _ repository.DBHealthRepository = (*DBHealthStore)(nil)
