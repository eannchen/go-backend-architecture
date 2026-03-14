package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
	goredis "github.com/redis/go-redis/v9"

	httpdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/binding"
	authhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/auth"
	healthhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/health"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/observability"
	sessionmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/session"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	rediscachestore "github.com/eannchen/go-backend-architecture/internal/infra/cache/redis/store"
	postgresstore "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/store"
	"github.com/eannchen/go-backend-architecture/internal/infra/external/oauth"
	"github.com/eannchen/go-backend-architecture/internal/infra/external/otp"
	rediskvstore "github.com/eannchen/go-backend-architecture/internal/infra/kvstore/redis/store"
	composeduser "github.com/eannchen/go-backend-architecture/internal/infra/composed/user"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repocache "github.com/eannchen/go-backend-architecture/internal/repository/cache"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// wiring centralizes shared dependencies used when wiring constructors.
type wiring struct {
	cfg    config.Config
	log    logger.Logger
	tracer observability.Tracer
	meter  observability.Meter
}

type appRepositories struct {
	dbHealthRepo     repodb.DBHealthRepository
	txManager        repodb.TxManager
	cacheHealthStore repocache.CacheHealthStore
	kvHealthStore    repokvstore.KVHealthStore
	userRepo         repodb.UserRepository
	sessionRepo      repokvstore.SessionRepository
	otpRepo          repokvstore.OTPRepository
	oauthStateRepo   repokvstore.OAuthStateRepository
}

type appUsecases struct {
	health         usecasehealth.Usecase
	otpAuth        authotp.OTPAuthenticator
	oauthAuth      authoauth.OAuthAuthenticator
	sessionManager authsession.SessionManager
}

type appHandlers struct {
	health httpdelivery.RouteRegistrar
	auth   httpdelivery.RouteRegistrar
}

type redisStores struct {
	cacheHealth repocache.CacheHealthStore
	userCache   *rediscachestore.UserCacheStore
	kvHealth    repokvstore.KVHealthStore
	session     *rediskvstore.SessionStore
	otp         *rediskvstore.OTPStore
	oauthState  *rediskvstore.OAuthStateStore
}

func newWiring(cfg config.Config, log logger.Logger, tracer observability.Tracer, meter observability.Meter) wiring {
	return wiring{
		cfg:    cfg,
		log:    log,
		tracer: tracer,
		meter:  meter,
	}
}

func (d wiring) buildRedisStores(client *goredis.Client) redisStores {
	return redisStores{
		cacheHealth: rediscachestore.NewHealthStore(client),
		userCache:   rediscachestore.NewUserCacheStore(client, d.cfg.Redis.CacheTTL),
		kvHealth:    rediskvstore.NewHealthStore(client),
		session:     rediskvstore.NewSessionStore(client),
		otp:         rediskvstore.NewOTPStore(client),
		oauthState:  rediskvstore.NewOAuthStateStore(client),
	}
}

func (d wiring) buildRepositories(pool *pgxpool.Pool, redis redisStores) appRepositories {
	dbUserRepo := postgresstore.NewUserStore(pool, d.tracer)
	userRepo := composeduser.NewCachedUserStore(d.log, d.tracer, dbUserRepo, redis.userCache)

	return appRepositories{
		txManager:        postgres.NewTxManager(pool, d.tracer),
		dbHealthRepo:     postgresstore.NewDBHealthStore(pool, d.tracer),
		cacheHealthStore: redis.cacheHealth,
		kvHealthStore:    redis.kvHealth,
		userRepo:         userRepo,
		sessionRepo:      redis.session,
		otpRepo:          redis.otp,
		oauthStateRepo:   redis.oauthState,
	}
}

func (d wiring) buildOAuthProviders() []repoexternal.OAuthProvider {
	var providers []repoexternal.OAuthProvider
	gcfg := d.cfg.Auth.OAuth.Google
	if gcfg.ClientID != "" {
		providers = append(providers, oauth.NewGoogleProvider(oauth.GoogleConfig{
			ClientID:     gcfg.ClientID,
			ClientSecret: gcfg.ClientSecret,
			RedirectURL:  gcfg.RedirectURL,
		}))
	}
	return providers
}

func (d wiring) buildUsecases(repos appRepositories) appUsecases {
	emailSender := otp.NewStubSender(d.log)

	return appUsecases{
		health: usecasehealth.New(d.tracer, d.meter, repos.dbHealthRepo, repos.cacheHealthStore, repos.kvHealthStore),
		sessionManager: authsession.NewServerSessionManager(
			d.tracer, d.meter, repos.sessionRepo, d.cfg.Auth.Session.TTL,
		),
		otpAuth: authotp.NewOTPAuthenticator(
			d.log, d.tracer, d.meter,
			repos.otpRepo, emailSender, repos.userRepo,
			authotp.OTPConfig{
				CodeLength: d.cfg.Auth.OTP.CodeLength,
				TTL:        d.cfg.Auth.OTP.TTL,
			},
		),
		oauthAuth: authoauth.NewOAuthAuthenticator(
			d.tracer, d.meter,
			repos.oauthStateRepo, repos.userRepo,
			d.buildOAuthProviders()...,
		),
	}
}

func (d wiring) buildHandlers(responder httpresponse.Responder, usecases appUsecases) appHandlers {
	return appHandlers{
		health: healthhttp.NewHandler(d.log, d.tracer, responder, usecases.health),
		auth: authhttp.NewHandler(
			d.log, d.tracer, responder,
			usecases.otpAuth, usecases.oauthAuth, usecases.sessionManager,
			authhttp.SessionCookieConfig{
				Name:   d.cfg.Auth.Session.CookieName,
				Secure: d.cfg.Auth.Session.CookieSecure,
				TTL:    d.cfg.Auth.Session.TTL,
			},
		),
	}
}

func (d wiring) buildServer(responder httpresponse.Responder, handlers appHandlers, usecases appUsecases) (*httpdelivery.Server, error) {
	validatorRegistrars := []httpdelivery.ValidationRegistrar{
		healthhttp.RegisterValidation,
	}

	sessMW := sessionmw.New(usecases.sessionManager, d.cfg.Auth.Session.CookieName, responder)
	_ = sessMW // available for route groups that need auth; see RegisterRoutes

	middlewares := []echo.MiddlewareFunc{
		echoMiddleware.Recover(),
		contextmw.NewRequestContextMiddleware(d.cfg.HTTP.ReadTimeout, responder).Handler(),
		observabilitymw.New(d.tracer, d.log).Handler(),
	}
	serverCfg := httpdelivery.ServerConfig{
		Address:      d.cfg.HTTP.Address,
		ReadTimeout:  d.cfg.HTTP.ReadTimeout,
		WriteTimeout: d.cfg.HTTP.WriteTimeout,
		IdleTimeout:  d.cfg.HTTP.IdleTimeout,
	}
	binder := binding.NewNormalizeBinder(nil)
	return httpdelivery.NewServer(serverCfg, d.log, binder, validatorRegistrars, middlewares, handlers.health, handlers.auth)
}
