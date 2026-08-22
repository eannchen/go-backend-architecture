package grpcapi

import (
	"context"
	"errors"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	grpcdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/grpc"
	healthservice "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/service/health"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

type App struct {
	*appruntime.Runtime
	Server   *grpcdelivery.Server
	Reporter *healthservice.Reporter
}

var _ appruntime.Application = (*App)(nil)

func New(ctx context.Context) (*App, error) {
	runtime, err := appruntime.New(ctx)
	if err != nil {
		return nil, err
	}

	wiring := newWiring(
		runtime.Config,
		runtime.Logger,
		runtime.Observability.Tracer(),
		runtime.Observability.Meter(),
	)
	healthUsecase := wiring.buildHealthUsecase(runtime.DBPool, runtime.RedisClient)
	components, err := wiring.buildServer(healthUsecase)
	if err != nil {
		return nil, errutil.Join(
			err,
			errutil.Step("shutdown runtime after gRPC server init failure", runtime.Shutdown(ctx)),
		)
	}

	return &App{
		Runtime:  runtime,
		Server:   components.server,
		Reporter: components.reporter,
	}, nil
}

func (a *App) Start() error {
	a.Reporter.Start()
	return a.Server.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	var shutdownErr error
	if err := a.Reporter.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	if err := a.Server.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	if err := a.Runtime.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	return shutdownErr
}
