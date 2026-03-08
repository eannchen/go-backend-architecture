package store

import (
	"context"

	dbsqlc "go-backend-architecture/internal/infra/db/postgres/sqlc/gen"
	"go-backend-architecture/internal/infra/observability"
	"go-backend-architecture/internal/repository"
)

type AccountSummaryStore struct {
	queries *dbsqlc.Queries
	tracer  observability.Tracer
}

func NewAccountSummaryStore(db dbsqlc.DBTX, tracer observability.Tracer) *AccountSummaryStore {
	return &AccountSummaryStore{
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

var _ repository.AccountSummaryRepository = (*AccountSummaryStore)(nil)
