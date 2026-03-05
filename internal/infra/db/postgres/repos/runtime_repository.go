package repos

import (
	"context"
	"fmt"

	"vocynex-api/internal/infra/db/builder"
	"vocynex-api/internal/infra/observability"
	dbsqlc "vocynex-api/internal/infra/db/postgres/sqlc/gen"
	"vocynex-api/internal/repository"
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

func (r *RuntimeRepository) Ping(ctx context.Context) error {
	ctx, span := r.tracer.Start(ctx, "repository", "runtime_repository.ping",
		observability.KV{Key: "db.system", Value: "postgresql"},
		observability.KV{Key: "db.operation", Value: "ping"},
	)
	defer span.End()

	_, err := r.queries.Ping(ctx)
	if err != nil {
		span.Fail(err, err.Error())
		return err
	}
	span.OK()
	return err
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

func (r *RuntimeRepository) SearchRuntimeValues(ctx context.Context, prefix string, limit uint64) ([]repository.RuntimeKV, error) {
	ctx, span := r.tracer.Start(ctx, "repository", "runtime_repository.search_runtime_values",
		observability.KV{Key: "db.system", Value: "postgresql"},
		observability.KV{Key: "db.operation", Value: "select"},
		observability.KV{Key: "db.sql.table", Value: "system_runtime_kv"},
		observability.KV{Key: "query.prefix", Value: prefix},
	)
	defer span.End()

	if limit == 0 {
		limit = 50
	}
	span.SetAttributes(observability.KV{Key: "query.limit", Value: int64(limit)})

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
		span.Fail(err, "build query failed")
		return nil, fmt.Errorf("build dynamic runtime query: %w", err)
	}
	span.SetAttributes(observability.KV{Key: "db.statement", Value: sqlStr})

	rows, err := r.db.Query(ctx, sqlStr, args...)
	if err != nil {
		span.Fail(err, "query execution failed")
		return nil, fmt.Errorf("execute dynamic runtime query: %w", err)
	}
	defer rows.Close()

	result := make([]repository.RuntimeKV, 0, limit)
	for rows.Next() {
		var item repository.RuntimeKV
		if err := rows.Scan(&item.Key, &item.Value); err != nil {
			span.Fail(err, "row scan failed")
			return nil, fmt.Errorf("scan dynamic runtime row: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		span.Fail(err, "row iteration failed")
		return nil, fmt.Errorf("iterate dynamic runtime rows: %w", err)
	}

	span.OK()
	return result, nil
}
