package store

import (
	"context"
	"fmt"

	"go-backend-architecture/internal/infra/db/builder"
	dbsqlc "go-backend-architecture/internal/infra/db/postgres/sqlc/gen"
	"go-backend-architecture/internal/infra/observability"
	"go-backend-architecture/internal/repository"
)

type RuntimeRepository struct {
	db      dbsqlc.DBTX
	queries *dbsqlc.Queries
	tracer  observability.Tracer
}

func NewRuntimeRepository(db dbsqlc.DBTX, tracer observability.Tracer) *RuntimeRepository {
	return &RuntimeRepository{
		db:      db,
		queries: dbsqlc.New(db),
		tracer:  tracer,
	}
}

func (r *RuntimeRepository) Ping(ctx context.Context) (err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "runtime_repository.ping",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "ping",
		),
	)
	defer func() {
		span.Finish(err)
	}()

	_, err = r.queries.Ping(ctx)
	return err
}

func (r *RuntimeRepository) GetServerStatus(ctx context.Context) (status repository.DBServerStatus, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "runtime_repository.get_server_status",
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
		return repository.DBServerStatus{}, err
	}

	status = repository.DBServerStatus{
		DatabaseName:  row.DatabaseName,
		InRecovery:    row.InRecovery,
		UptimeSeconds: row.UptimeSeconds,
	}
	return status, nil
}

func (r *RuntimeRepository) GetRuntimeValue(ctx context.Context, key string) (repository.RuntimeKV, error) {
	row, err := r.queries.GetRuntimeValue(ctx, key)
	if err != nil {
		return repository.RuntimeKV{}, err
	}

	return repository.RuntimeKV{
		Key:   row.Key,
		Value: row.Value,
	}, nil
}

func (r *RuntimeRepository) SearchRuntimeValues(ctx context.Context, prefix string, limit uint64) (result []repository.RuntimeKV, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "runtime_repository.search_runtime_values",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "select",
			"db.sql.table", "system_runtime_kv",
			"query.prefix", prefix,
		),
	)
	defer func() {
		span.Finish(err)
	}()

	if limit == 0 {
		limit = 50
	}
	span.SetAttributes(observability.FromPairs("query.limit", int64(limit)))

	query := builder.StatementBuilder.
		Select("key", "value").
		From("system_runtime_kv").
		OrderBy("key ASC").
		Limit(limit)

	if prefix != "" {
		query = query.Where("key ILIKE ?", prefix+"%")
	}

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build dynamic runtime query: %w", err)
	}
	span.SetAttributes(observability.FromPairs("db.statement", sqlStr))

	rows, err := r.db.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("execute dynamic runtime query: %w", err)
	}
	defer rows.Close()

	result = make([]repository.RuntimeKV, 0, limit)
	for rows.Next() {
		var item repository.RuntimeKV
		if err := rows.Scan(&item.Key, &item.Value); err != nil {
			return nil, fmt.Errorf("scan dynamic runtime row: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dynamic runtime rows: %w", err)
	}

	return result, nil
}
