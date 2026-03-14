package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

const oauthStateTTL = 10 * time.Minute

// OAuthAuthenticator handles OAuth2 authorization-code flows.
// Implementations manage provider selection, state tokens, and code exchange.
//
// Strategy examples:
//   - Multi-provider: registry of OAuthProvider implementations keyed by name.
//   - Single-provider: hardcoded to one provider.
type OAuthAuthenticator interface {
	AuthorizeURL(ctx context.Context, provider string) (redirectURL string, err error)
	HandleCallback(ctx context.Context, provider, code, state string) (auth.Identity, error)
}

type oauthAuthenticator struct {
	tracer     observability.Tracer
	providers  map[string]repoexternal.OAuthProvider
	stateRepo  repokvstore.OAuthStateRepository
	userRepo   repodb.UserRepository
	oauthTotal observability.Counter
}

// NewOAuthAuthenticator creates an OAuth authenticator with a registry of providers.
func NewOAuthAuthenticator(
	tracer observability.Tracer,
	meter observability.Meter,
	stateRepo repokvstore.OAuthStateRepository,
	userRepo repodb.UserRepository,
	providers ...repoexternal.OAuthProvider,
) OAuthAuthenticator {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	if meter == nil {
		meter = observability.NoopMeter{}
	}

	pm := make(map[string]repoexternal.OAuthProvider, len(providers))
	for _, p := range providers {
		pm[p.Name()] = p
	}

	return &oauthAuthenticator{
		tracer:     tracer,
		providers:  pm,
		stateRepo:  stateRepo,
		userRepo:   userRepo,
		oauthTotal: meter.Counter("auth_oauth_total",
			observability.MetricOption{Description: "OAuth flow completions by provider and outcome.", Unit: "{attempt}"},
		),
	}
}

func (a *oauthAuthenticator) AuthorizeURL(ctx context.Context, provider string) (redirectURL string, err error) {
	ctx, span := a.tracer.Start(ctx, "usecase", "oauth_authenticator.authorize_url",
		observability.FromPairs("oauth.provider", provider),
	)
	defer func() { span.Finish(err) }()

	p, ok := a.providers[provider]
	if !ok {
		return "", apperr.New(apperr.CodeInvalidArgument, "unsupported oauth provider: "+provider)
	}

	state, err := auth.GenerateToken(16)
	if err != nil {
		return "", apperr.Wrap(err, apperr.CodeInternal, "generate oauth state")
	}

	if err := a.stateRepo.Store(ctx, state, oauthStateTTL); err != nil {
		return "", apperr.Wrap(err, apperr.CodeInternal, "store oauth state")
	}

	return p.AuthCodeURL(state), nil
}

func (a *oauthAuthenticator) HandleCallback(ctx context.Context, provider, code, state string) (identity auth.Identity, err error) {
	ctx, span := a.tracer.Start(ctx, "usecase", "oauth_authenticator.handle_callback",
		observability.FromPairs("oauth.provider", provider),
	)
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		a.oauthTotal.Add(ctx, 1, observability.FromPairs("oauth.provider", provider, "outcome", outcome))
		span.Finish(err)
	}()

	valid, err := a.stateRepo.Consume(ctx, state)
	if err != nil {
		return auth.Identity{}, apperr.Wrap(err, apperr.CodeInternal, "validate oauth state")
	}
	if !valid {
		return auth.Identity{}, apperr.New(apperr.CodeUnauthorized, "invalid or expired oauth state")
	}

	p, ok := a.providers[provider]
	if !ok {
		return auth.Identity{}, apperr.New(apperr.CodeInvalidArgument, "unsupported oauth provider: "+provider)
	}

	userInfo, err := p.Exchange(ctx, code)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return auth.Identity{}, apperr.Wrap(err, apperr.CodeTimeout, "oauth code exchange timed out")
		}
		return auth.Identity{}, apperr.Wrap(err, apperr.CodeUnauthorized, "oauth code exchange failed")
	}

	user, err := a.userRepo.UpsertOAuthUser(ctx, repodb.OAuthUserUpsert{
		Provider:       provider,
		ProviderUserID: userInfo.ProviderUserID,
		Email:          userInfo.Email,
	})
	if err != nil {
		return auth.Identity{}, apperr.Wrap(err, apperr.CodeInternal, "find or create oauth user")
	}

	return auth.Identity{
		UserID: user.ID,
		Email:  user.Email,
		Method: auth.MethodOAuth,
	}, nil
}
