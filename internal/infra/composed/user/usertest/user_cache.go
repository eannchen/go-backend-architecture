package usertest

import (
	"context"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
)

// UserCache is a configurable double for its parent package's UserCache contract.
type UserCache struct {
	GetByIDFunc     func(ctx context.Context, id int64) (domainuser.User, bool, error)
	GetByIDCalls    []UserCacheGetByIDCall
	SetByIDFunc     func(ctx context.Context, id int64, user domainuser.User) error
	SetByIDCalls    []UserCacheSetByIDCall
	DeleteByIDFunc  func(ctx context.Context, id int64) error
	DeleteByIDCalls []UserCacheDeleteByIDCall
}

// UserCacheGetByIDCall records one UserCache.GetByID invocation.
type UserCacheGetByIDCall struct {
	Context context.Context
	ID      int64
}

func (d *UserCache) GetByID(ctx context.Context, id int64) (domainuser.User, bool, error) {
	d.GetByIDCalls = append(d.GetByIDCalls, UserCacheGetByIDCall{Context: ctx, ID: id})
	fn := d.GetByIDFunc
	if fn == nil {
		panic("unexpected UserCache.GetByID call")
	}
	return fn(ctx, id)
}

// UserCacheSetByIDCall records one UserCache.SetByID invocation.
type UserCacheSetByIDCall struct {
	Context context.Context
	ID      int64
	User    domainuser.User
}

func (d *UserCache) SetByID(ctx context.Context, id int64, user domainuser.User) error {
	d.SetByIDCalls = append(d.SetByIDCalls, UserCacheSetByIDCall{Context: ctx, ID: id, User: user})
	fn := d.SetByIDFunc
	if fn == nil {
		panic("unexpected UserCache.SetByID call")
	}
	return fn(ctx, id, user)
}

// UserCacheDeleteByIDCall records one UserCache.DeleteByID invocation.
type UserCacheDeleteByIDCall struct {
	Context context.Context
	ID      int64
}

func (d *UserCache) DeleteByID(ctx context.Context, id int64) error {
	d.DeleteByIDCalls = append(d.DeleteByIDCalls, UserCacheDeleteByIDCall{Context: ctx, ID: id})
	fn := d.DeleteByIDFunc
	if fn == nil {
		panic("unexpected UserCache.DeleteByID call")
	}
	return fn(ctx, id)
}
