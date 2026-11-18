package user

import "context"

// Repository stores users independently of the database and authentication method.
type Repository interface {
	GetByEmail(context.Context, string) (User, error)
	GetByID(context.Context, int64) (User, error)
	CreateByEmail(context.Context, string) (User, error)
}
