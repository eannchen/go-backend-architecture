package store

import (
	"context"
	"fmt"

	dbsqlc "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/sqlc/gen"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
	"github.com/eannchen/go-backend-architecture/pkg/observability"
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
		span.Finish(observability.SpanOutcome{Err: err, Failed: err != nil})
	}()

	_, err = r.queries.Ping(ctx)
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

func (r *DBHealthStore) GetServerStatus(ctx context.Context) (status usecasehealth.DBServerStatus, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "db_health_store.get_server_status",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "select",
			"db.statement.type", "server_status",
		),
	)
	defer func() {
		span.Finish(observability.SpanOutcome{Err: err, Failed: err != nil})
	}()

	row, err := r.queries.GetServerStatus(ctx)
	if err != nil {
		return usecasehealth.DBServerStatus{}, fmt.Errorf("database server status query failed: %w", err)
	}

	status = usecasehealth.DBServerStatus{
		DatabaseName:  row.DatabaseName,
		InRecovery:    row.InRecovery,
		UptimeSeconds: row.UptimeSeconds,
	}
	return status, nil
}
