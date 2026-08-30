package dbtest

import (
	"context"

	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

// UserRepository is a reusable test double for repodb.UserRepository.
type UserRepository struct {
	GetByEmailFunc       func(context.Context, string) (repodb.User, error)
	GetByEmailCalls      int
	GetByEmailEmail      string
	GetByIDFunc          func(context.Context, int64) (repodb.User, error)
	GetByIDCalls         int
	GetByIDID            int64
	CreateByEmailFunc    func(context.Context, string) (repodb.User, error)
	CreateByEmailCalls   int
	CreateByEmailEmail   string
	UpsertOAuthUserFunc  func(context.Context, repodb.OAuthUserUpsert) (repodb.User, error)
	UpsertOAuthUserCalls int
	UpsertOAuthUserInfo  repodb.OAuthUserUpsert
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (repodb.User, error) {
	r.GetByEmailCalls++
	r.GetByEmailEmail = email
	if r.GetByEmailFunc == nil {
		panic("unexpected UserRepository.GetByEmail call")
	}
	return r.GetByEmailFunc(ctx, email)
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (repodb.User, error) {
	r.GetByIDCalls++
	r.GetByIDID = id
	if r.GetByIDFunc == nil {
		panic("unexpected UserRepository.GetByID call")
	}
	return r.GetByIDFunc(ctx, id)
}

func (r *UserRepository) CreateByEmail(ctx context.Context, email string) (repodb.User, error) {
	r.CreateByEmailCalls++
	r.CreateByEmailEmail = email
	if r.CreateByEmailFunc == nil {
		panic("unexpected UserRepository.CreateByEmail call")
	}
	return r.CreateByEmailFunc(ctx, email)
}

func (r *UserRepository) UpsertOAuthUser(ctx context.Context, info repodb.OAuthUserUpsert) (repodb.User, error) {
	r.UpsertOAuthUserCalls++
	r.UpsertOAuthUserInfo = info
	if r.UpsertOAuthUserFunc == nil {
		panic("unexpected UserRepository.UpsertOAuthUser call")
	}
	return r.UpsertOAuthUserFunc(ctx, info)
}

var _ repodb.UserRepository = (*UserRepository)(nil)
