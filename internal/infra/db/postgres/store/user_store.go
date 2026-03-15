package store

import (
	"context"
	"errors"
	"fmt"

	dbsqlc "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/sqlc/gen"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

// UserStore implements UserRepository using Postgres via sqlc (users table).
type UserStore struct {
	queries *dbsqlc.Queries
	tracer  observability.Tracer
}

func NewUserStore(db dbsqlc.DBTX, tracer observability.Tracer) *UserStore {
	return &UserStore{
		queries: dbsqlc.New(db),
		tracer:  tracer,
	}
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (user repodb.User, err error) {
	ctx, span := s.tracer.Start(ctx, "repository", "user_store.get_by_email",
		observability.FromPairs("db.system", "postgresql", "db.operation", "select", "db.sql.table", "users"),
	)
	defer func() { span.Finish(err) }()

	row, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return repodb.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return repodb.User{ID: int64(row.ID), Email: row.Email}, nil
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (user repodb.User, err error) {
	ctx, span := s.tracer.Start(ctx, "repository", "user_store.get_by_id",
		observability.FromPairs("db.system", "postgresql", "db.operation", "select", "db.sql.table", "users", "user.id", id),
	)
	defer func() { span.Finish(err) }()

	row, err := s.queries.GetUserByID(ctx, int32(id))
	if err != nil {
		return repodb.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return repodb.User{ID: int64(row.ID), Email: row.Email}, nil
}

func (s *UserStore) CreateByEmail(ctx context.Context, email string) (user repodb.User, err error) {
	ctx, span := s.tracer.Start(ctx, "repository", "user_store.create_by_email",
		observability.FromPairs("db.system", "postgresql", "db.operation", "insert", "db.sql.table", "users"),
	)
	defer func() { span.Finish(err) }()

	row, err := s.queries.CreateUser(ctx, email)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return repodb.User{}, fmt.Errorf("create user: %w", errors.Join(repodb.ErrDuplicateKey, err))
		}
		return repodb.User{}, fmt.Errorf("create user: %w", err)
	}
	return repodb.User{ID: int64(row.ID), Email: row.Email}, nil
}

func (s *UserStore) UpsertOAuthUser(ctx context.Context, info repodb.OAuthUserUpsert) (user repodb.User, err error) {
	ctx, span := s.tracer.Start(ctx, "repository", "user_store.upsert_oauth_user",
		observability.FromPairs("db.system", "postgresql", "db.operation", "upsert", "db.sql.table", "users", "oauth.provider", info.Provider),
	)
	defer func() { span.Finish(err) }()

	row, err := s.queries.UpsertOAuthConnection(ctx, dbsqlc.UpsertOAuthConnectionParams{
		Provider:       info.Provider,
		ProviderUserID: info.ProviderUserID,
		Email:          info.Email,
	})
	if err != nil {
		return repodb.User{}, fmt.Errorf("upsert oauth connection: %w", err)
	}
	return repodb.User{ID: int64(row.ID), Email: row.Email}, nil
}

var _ repodb.UserRepository = (*UserStore)(nil)
