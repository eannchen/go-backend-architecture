package api

import (
	"context"
	"errors"

	goredis "github.com/redis/go-redis/v9"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	httpDelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/infra/redisconn"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

type App struct {
	*appruntime.Runtime
	RedisClient *goredis.Client
	Server      *httpDelivery.Server
}

var _ appruntime.Application = (*App)(nil)

func New(ctx context.Context) (*App, error) {
	runtime, err := appruntime.New(ctx)
	if err != nil {
		return nil, err
	}
	wiring := newWiring(runtime.Config, runtime.Logger, runtime.Observability.Tracer(), runtime.Observability.Meter())
	redisClient := redisconn.NewClient(runtime.Config.Redis)
	redisStores := wiring.buildRedisStores(redisClient)

	repositories := wiring.buildRepositories(runtime.DBPool, redisStores)
	usecases := wiring.buildUsecases(repositories)
	responder := httpresponse.NewResponder(httpcontext.NewContextMeta())
	handlers := wiring.buildHandlers(responder, usecases)
	server, err := wiring.buildServer(responder, repositories, handlers, usecases)
	if err != nil {
		return nil, errutil.Join(
			err,
			errutil.Step("close redis client after server init failure", closeRedisWithError(ctx, redisClient)),
			errutil.Step("shutdown runtime after server init failure", runtime.Shutdown(ctx)),
		)
	}

	return &App{
		Runtime:     runtime,
		RedisClient: redisClient,
		Server:      server,
	}, nil
}

func (a *App) Start() error {
	return a.Server.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	var shutdownErr error

	if err := a.Server.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if err := closeRedisWithError(ctx, a.RedisClient); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	if err := a.Runtime.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	return shutdownErr
}

func closeRedisWithError(ctx context.Context, client *goredis.Client) error {
	if client == nil {
		return nil
	}
	return client.Close()
}
