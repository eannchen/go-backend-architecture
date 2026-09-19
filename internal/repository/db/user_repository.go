package db

import (
	"context"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
)

// OAuthUserUpsert holds provider identity data used to find or create a user during OAuth login.
type OAuthUserUpsert struct {
	Provider       string
	ProviderUserID string
	Email          string
}

// UserRepository provides user lookups and creation for authentication flows.
type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (domainuser.User, error)
	GetByID(ctx context.Context, id int64) (domainuser.User, error)
	CreateByEmail(ctx context.Context, email string) (domainuser.User, error)
	UpsertOAuthUser(ctx context.Context, info OAuthUserUpsert) (domainuser.User, error)
}
