package cache

import (
	"context"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
)

// UserCacheStore caches user lookups to reduce database load.
type UserCacheStore interface {
	GetByID(ctx context.Context, id int64) (user domainuser.User, found bool, err error)
	SetByID(ctx context.Context, id int64, user domainuser.User) error
	DeleteByID(ctx context.Context, id int64) error
}
