package user

import (
	"context"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// UserStore supplies the operations this decorator forwards to primary storage.
type UserStore interface {
	domainuser.Repository
	authoauth.UserRepository
}

// UserCache supplies the cache operations used by this decorator.
type UserCache interface {
	GetByID(context.Context, int64) (domainuser.User, bool, error)
	SetByID(context.Context, int64, domainuser.User) error
	DeleteByID(context.Context, int64) error
}
