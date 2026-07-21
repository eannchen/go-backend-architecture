package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/oauth"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// BrowserBindingTTL bounds the one-time browser cookie, OAuth state, and PKCE
// verifier. Keeping their lifetimes equal avoids a valid state outliving its
// browser proof.
const BrowserBindingTTL = 10 * time.Minute

// OAuthAuthenticator handles OAuth2 authorization-code flows.
// Implementations manage provider selection, state tokens, and code exchange.
//
// Strategy examples:
//   - Multi-provider: registry of OAuthProvider implementations keyed by name.
//   - Single-provider: hardcoded to one provider.
type OAuthAuthenticator interface {
	Authorize(ctx context.Context, provider string) (Authorization, error)
	HandleCallback(ctx context.Context, provider, code, state, browserBinding string) (auth.Identity, error)
}

// Authorization contains the provider redirect and the browser-only secret that
// must return in the callback cookie. BrowserBinding is also the PKCE verifier.
type Authorization struct {
	RedirectURL    string
	BrowserBinding string
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
		tracer:    tracer,
		providers: pm,
		stateRepo: stateRepo,
		userRepo:  userRepo,
		oauthTotal: meter.Counter("auth_oauth_total",
			observability.MetricOption{Description: "OAuth flow completions by provider and outcome.", Unit: "{attempt}"},
		),
	}
}

func (a *oauthAuthenticator) Authorize(ctx context.Context, provider string) (authorization Authorization, err error) {
	ctx, span := a.tracer.Start(ctx, "usecase", "oauth_authenticator.authorize_url",
		observability.FromPairs("oauth.provider", provider),
	)
	defer func() { span.Finish(err) }()

	p, ok := a.providers[provider]
	if !ok {
		return Authorization{}, apperr.New(apperr.CodeInvalidArgument, "unsupported oauth provider: "+provider)
	}

	state, err := auth.GenerateToken(16)
	if err != nil {
		return Authorization{}, apperr.Wrap(err, apperr.CodeInternal, "generate oauth state")
	}
	browserBinding, err := auth.GenerateToken(32)
	if err != nil {
		return Authorization{}, apperr.Wrap(err, apperr.CodeInternal, "generate oauth browser binding")
	}

	if err := a.stateRepo.Store(ctx, state, repokvstore.OAuthStateData{
		BrowserBindingHash: hashBrowserBinding(browserBinding),
	}, BrowserBindingTTL); err != nil {
		return Authorization{}, apperr.Wrap(err, apperr.CodeInternal, "store oauth state")
	}

	return Authorization{
		RedirectURL:    p.AuthCodeURL(state, browserBinding),
		BrowserBinding: browserBinding,
	}, nil
}

func (a *oauthAuthenticator) HandleCallback(ctx context.Context, provider, code, state, browserBinding string) (identity auth.Identity, err error) {
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

	p, ok := a.providers[provider]
	if !ok {
		return auth.Identity{}, apperr.New(apperr.CodeInvalidArgument, "unsupported oauth provider: "+provider)
	}
	if browserBinding == "" {
		return auth.Identity{}, apperr.New(apperr.CodeUnauthorized, "oauth browser binding is missing or expired")
	}

	stateData, found, err := a.stateRepo.Consume(ctx, state)
	if err != nil {
		return auth.Identity{}, apperr.Wrap(err, apperr.CodeInternal, "validate oauth state")
	}
	if !found {
		return auth.Identity{}, apperr.New(apperr.CodeUnauthorized, "invalid or expired oauth state")
	}
	if subtle.ConstantTimeCompare(
		[]byte(stateData.BrowserBindingHash),
		[]byte(hashBrowserBinding(browserBinding)),
	) != 1 {
		return auth.Identity{}, apperr.New(apperr.CodeUnauthorized, "oauth browser binding is invalid or expired")
	}

	userInfo, err := p.Exchange(ctx, code, browserBinding)
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

func hashBrowserBinding(binding string) string {
	sum := sha256.Sum256([]byte(binding))
	return hex.EncodeToString(sum[:])
}
