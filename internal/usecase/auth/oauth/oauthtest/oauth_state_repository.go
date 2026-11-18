package oauthtest

import (
	"context"
	"time"

	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
)

// OAuthStateRepository is a configurable double for its parent package's OAuthStateRepository contract.
type OAuthStateRepository struct {
	StoreFunc    func(ctx context.Context, state string, data authoauth.OAuthStateData, ttl time.Duration) error
	StoreCalls   []OAuthStateRepositoryStoreCall
	ConsumeFunc  func(ctx context.Context, state string) (authoauth.OAuthStateData, bool, error)
	ConsumeCalls []OAuthStateRepositoryConsumeCall
}

// OAuthStateRepositoryStoreCall records one OAuthStateRepository.Store invocation.
type OAuthStateRepositoryStoreCall struct {
	Context context.Context
	State   string
	Data    authoauth.OAuthStateData
	TTL     time.Duration
}

func (d *OAuthStateRepository) Store(ctx context.Context, state string, data authoauth.OAuthStateData, ttl time.Duration) error {
	d.StoreCalls = append(d.StoreCalls, OAuthStateRepositoryStoreCall{Context: ctx, State: state, Data: data, TTL: ttl})
	fn := d.StoreFunc
	if fn == nil {
		panic("unexpected OAuthStateRepository.Store call")
	}
	return fn(ctx, state, data, ttl)
}

// OAuthStateRepositoryConsumeCall records one OAuthStateRepository.Consume invocation.
type OAuthStateRepositoryConsumeCall struct {
	Context context.Context
	State   string
}

func (d *OAuthStateRepository) Consume(ctx context.Context, state string) (authoauth.OAuthStateData, bool, error) {
	d.ConsumeCalls = append(d.ConsumeCalls, OAuthStateRepositoryConsumeCall{Context: ctx, State: state})
	fn := d.ConsumeFunc
	if fn == nil {
		panic("unexpected OAuthStateRepository.Consume call")
	}
	return fn(ctx, state)
}
