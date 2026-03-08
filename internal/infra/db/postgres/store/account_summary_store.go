package store

import (
	"context"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"

	"go-backend-architecture/internal/infra/db/builder"
	dbsqlc "go-backend-architecture/internal/infra/db/postgres/sqlc/gen"
	"go-backend-architecture/internal/infra/observability"
	"go-backend-architecture/internal/repository"
)

type AccountSummaryStore struct {
	db      dbsqlc.DBTX
	queries *dbsqlc.Queries
	tracer  observability.Tracer
}

func NewAccountSummaryStore(db dbsqlc.DBTX, tracer observability.Tracer) *AccountSummaryStore {
	return &AccountSummaryStore{
		db:      db,
		queries: dbsqlc.New(db),
		tracer:  tracer,
	}
}

func (r *AccountSummaryStore) GetByID(ctx context.Context, id int64) (summary repository.AccountSummary, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "account_summary_store.get_by_id",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "select",
			"db.sql.table", "account_summaries",
			"account.id", id,
		),
	)
	defer func() {
		span.Finish(err)
	}()

	row, err := r.queries.GetAccountSummaryByID(ctx, id)
	if err != nil {
		return repository.AccountSummary{}, err
	}
	return repository.AccountSummary{
		ID:          row.ID,
		Email:       row.Email,
		DisplayName: row.DisplayName,
		Plan:        row.Plan,
		Status:      row.Status,
		UpdatedAt:   row.UpdatedAtUnix,
	}, nil
}

func (r *AccountSummaryStore) Search(ctx context.Context, filter repository.AccountSummarySearchFilter) (items []repository.AccountSummary, err error) {
	ctx, span := r.tracer.Start(ctx, "repository", "account_summary_store.search",
		observability.FromPairs(
			"db.system", "postgresql",
			"db.operation", "select",
			"db.sql.table", "account_summaries",
		),
	)
	defer func() {
		span.Finish(err)
	}()

	qb := builder.StatementBuilder.
		Select(
			"id",
			"email",
			"display_name",
			"plan",
			"status",
			"EXTRACT(EPOCH FROM updated_at)::BIGINT AS updated_at_unix",
		).
		From("account_summaries")

	if filter.Plan != "" {
		qb = qb.Where(sq.Eq{"plan": filter.Plan})
	}
	if filter.Status != "" {
		qb = qb.Where(sq.Eq{"status": filter.Status})
	}
	if filter.EmailLike != "" {
		qb = qb.Where("email ILIKE ?", "%"+strings.TrimSpace(filter.EmailLike)+"%")
	}

	if strings.EqualFold(filter.SortUpdated, "asc") {
		qb = qb.OrderBy("updated_at ASC")
	} else {
		qb = qb.OrderBy("updated_at DESC")
	}
	if filter.Limit > 0 {
		qb = qb.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		qb = qb.Offset(filter.Offset)
	}

	sql, args, err := qb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build account summary search query: %w", err)
	}
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query account summaries: %w", err)
	}
	defer rows.Close()

	items = make([]repository.AccountSummary, 0, 8)
	for rows.Next() {
		var item repository.AccountSummary
		if err := rows.Scan(
			&item.ID,
			&item.Email,
			&item.DisplayName,
			&item.Plan,
			&item.Status,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account summary row: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate account summary rows: %w", err)
	}
	return items, nil
}

var _ repository.AccountSummaryRepository = (*AccountSummaryStore)(nil)
