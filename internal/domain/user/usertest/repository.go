package usertest

import (
	"context"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
)

// Repository is a configurable double for its parent package's Repository contract.
type Repository struct {
	GetByEmailFunc     func(ctx context.Context, email string) (domainuser.User, error)
	GetByEmailCalls    []RepositoryGetByEmailCall
	GetByIDFunc        func(ctx context.Context, id int64) (domainuser.User, error)
	GetByIDCalls       []RepositoryGetByIDCall
	CreateByEmailFunc  func(ctx context.Context, email string) (domainuser.User, error)
	CreateByEmailCalls []RepositoryCreateByEmailCall
}

// RepositoryGetByEmailCall records one Repository.GetByEmail invocation.
type RepositoryGetByEmailCall struct {
	Context context.Context
	Email   string
}

func (d *Repository) GetByEmail(ctx context.Context, email string) (domainuser.User, error) {
	d.GetByEmailCalls = append(d.GetByEmailCalls, RepositoryGetByEmailCall{Context: ctx, Email: email})
	fn := d.GetByEmailFunc
	if fn == nil {
		panic("unexpected Repository.GetByEmail call")
	}
	return fn(ctx, email)
}

// RepositoryGetByIDCall records one Repository.GetByID invocation.
type RepositoryGetByIDCall struct {
	Context context.Context
	ID      int64
}

func (d *Repository) GetByID(ctx context.Context, id int64) (domainuser.User, error) {
	d.GetByIDCalls = append(d.GetByIDCalls, RepositoryGetByIDCall{Context: ctx, ID: id})
	fn := d.GetByIDFunc
	if fn == nil {
		panic("unexpected Repository.GetByID call")
	}
	return fn(ctx, id)
}

// RepositoryCreateByEmailCall records one Repository.CreateByEmail invocation.
type RepositoryCreateByEmailCall struct {
	Context context.Context
	Email   string
}

func (d *Repository) CreateByEmail(ctx context.Context, email string) (domainuser.User, error) {
	d.CreateByEmailCalls = append(d.CreateByEmailCalls, RepositoryCreateByEmailCall{Context: ctx, Email: email})
	fn := d.CreateByEmailFunc
	if fn == nil {
		panic("unexpected Repository.CreateByEmail call")
	}
	return fn(ctx, email)
}
