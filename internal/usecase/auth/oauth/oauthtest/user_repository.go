package oauthtest

import (
	"context"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// UserRepository is a configurable double for its parent package's UserRepository contract.
type UserRepository struct {
	UpsertOAuthUserFunc  func(ctx context.Context, info authoauth.OAuthUserUpsert) (domainuser.User, error)
	UpsertOAuthUserCalls []UserRepositoryUpsertOAuthUserCall
}

// UserRepositoryUpsertOAuthUserCall records one UserRepository.UpsertOAuthUser invocation.
type UserRepositoryUpsertOAuthUserCall struct {
	Context context.Context
	Info    authoauth.OAuthUserUpsert
}

func (d *UserRepository) UpsertOAuthUser(ctx context.Context, info authoauth.OAuthUserUpsert) (domainuser.User, error) {
	d.UpsertOAuthUserCalls = append(d.UpsertOAuthUserCalls, UserRepositoryUpsertOAuthUserCall{Context: ctx, Info: info})
	fn := d.UpsertOAuthUserFunc
	if fn == nil {
		panic("unexpected UserRepository.UpsertOAuthUser call")
	}
	return fn(ctx, info)
}
