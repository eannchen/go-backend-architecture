package cache

import (
	"context"

	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

// UserCacheStore caches user lookups to reduce database load.
type UserCacheStore interface {
	GetByID(ctx context.Context, id int64) (user repodb.User, found bool, err error)
	SetByID(ctx context.Context, id int64, user repodb.User) error
	DeleteByID(ctx context.Context, id int64) error
}
