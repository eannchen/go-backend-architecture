package db

import "context"

// User is the minimal user representation used by the auth layer.
type User struct {
	ID    int64
	Email string
}

// OAuthUserUpsert holds provider identity data used to find or create a user during OAuth login.
type OAuthUserUpsert struct {
	Provider       string
	ProviderUserID string
	Email          string
}

// UserRepository provides user lookups and creation for authentication flows.
type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id int64) (User, error)
	CreateByEmail(ctx context.Context, email string) (User, error)
	UpsertOAuthUser(ctx context.Context, info OAuthUserUpsert) (User, error)
}
