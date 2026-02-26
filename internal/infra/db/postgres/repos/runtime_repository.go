package repos

import (
	"context"
	"fmt"

	"vocynex-api/internal/infra/db/builder"
	dbsqlc "vocynex-api/internal/infra/db/postgres/sqlc/gen"
	"vocynex-api/internal/repository"
)

type RuntimeRepository struct {
	db      dbsqlc.DBTX
	queries *dbsqlc.Queries
}

func NewRuntimeRepository(db dbsqlc.DBTX) *RuntimeRepository {
	return &RuntimeRepository{
		db:      db,
		queries: dbsqlc.New(db),
	}
}

func (r *RuntimeRepository) Ping(ctx context.Context) error {
	_, err := r.queries.Ping(ctx)
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
	if limit == 0 {
		limit = 50
	}

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

	rows, err := r.db.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("execute dynamic runtime query: %w", err)
	}
	defer rows.Close()

	result := make([]repository.RuntimeKV, 0, limit)
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
