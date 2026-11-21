package usertest

import (
	"context"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// UserStore is a configurable double for its parent package's UserStore contract.
type UserStore struct {
	GetByEmailFunc       func(ctx context.Context, email string) (domainuser.User, error)
	GetByEmailCalls      []UserStoreGetByEmailCall
	GetByIDFunc          func(ctx context.Context, id int64) (domainuser.User, error)
	GetByIDCalls         []UserStoreGetByIDCall
	CreateByEmailFunc    func(ctx context.Context, email string) (domainuser.User, error)
	CreateByEmailCalls   []UserStoreCreateByEmailCall
	UpsertOAuthUserFunc  func(ctx context.Context, info authoauth.OAuthUserUpsert) (domainuser.User, error)
	UpsertOAuthUserCalls []UserStoreUpsertOAuthUserCall
}

// UserStoreGetByEmailCall records one UserStore.GetByEmail invocation.
type UserStoreGetByEmailCall struct {
	Context context.Context
	Email   string
}

func (d *UserStore) GetByEmail(ctx context.Context, email string) (domainuser.User, error) {
	d.GetByEmailCalls = append(d.GetByEmailCalls, UserStoreGetByEmailCall{Context: ctx, Email: email})
	fn := d.GetByEmailFunc
	if fn == nil {
		panic("unexpected UserStore.GetByEmail call")
	}
	return fn(ctx, email)
}

// UserStoreGetByIDCall records one UserStore.GetByID invocation.
type UserStoreGetByIDCall struct {
	Context context.Context
	ID      int64
}

func (d *UserStore) GetByID(ctx context.Context, id int64) (domainuser.User, error) {
	d.GetByIDCalls = append(d.GetByIDCalls, UserStoreGetByIDCall{Context: ctx, ID: id})
	fn := d.GetByIDFunc
	if fn == nil {
		panic("unexpected UserStore.GetByID call")
	}
	return fn(ctx, id)
}

// UserStoreCreateByEmailCall records one UserStore.CreateByEmail invocation.
type UserStoreCreateByEmailCall struct {
	Context context.Context
	Email   string
}

func (d *UserStore) CreateByEmail(ctx context.Context, email string) (domainuser.User, error) {
	d.CreateByEmailCalls = append(d.CreateByEmailCalls, UserStoreCreateByEmailCall{Context: ctx, Email: email})
	fn := d.CreateByEmailFunc
	if fn == nil {
		panic("unexpected UserStore.CreateByEmail call")
	}
	return fn(ctx, email)
}

// UserStoreUpsertOAuthUserCall records one UserStore.UpsertOAuthUser invocation.
type UserStoreUpsertOAuthUserCall struct {
	Context context.Context
	Info    authoauth.OAuthUserUpsert
}

func (d *UserStore) UpsertOAuthUser(ctx context.Context, info authoauth.OAuthUserUpsert) (domainuser.User, error) {
	d.UpsertOAuthUserCalls = append(d.UpsertOAuthUserCalls, UserStoreUpsertOAuthUserCall{Context: ctx, Info: info})
	fn := d.UpsertOAuthUserFunc
	if fn == nil {
		panic("unexpected UserStore.UpsertOAuthUser call")
	}
	return fn(ctx, info)
}
