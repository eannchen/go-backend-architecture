package store

import (
	"context"
	"fmt"

	dbsqlc "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/sqlc/gen"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/repository"
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
		return fmt.Errorf("database ping failed: %w", err)
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
		return repository.DBServerStatus{}, fmt.Errorf("database server status query failed: %w", err)
	}

	status = repository.DBServerStatus{
		DatabaseName:  row.DatabaseName,
		InRecovery:    row.InRecovery,
		UptimeSeconds: row.UptimeSeconds,
	}
	return status, nil
}

func (r *DBHealthStore) CheckVectorExtension(ctx context.Context) (err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "db_health_store.check_vector_extension",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "select",
			"db.statement.type", "extension_check",
			"db.extension.name", "vector",
		),
	)
	defer func() {
		span.Finish(err)
	}()

	enabled, err := r.queries.IsVectorExtensionEnabled(ctx)
	if err != nil {
		return fmt.Errorf("vector extension check query failed: %w", err)
	}
	if !enabled {
		return fmt.Errorf("vector extension is not enabled")
	}
	return nil
}

var _ repository.DBHealthRepository = (*DBHealthStore)(nil)
