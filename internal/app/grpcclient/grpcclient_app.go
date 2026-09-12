package grpcclient

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	infragrpcclient "github.com/eannchen/go-backend-architecture/internal/infra/grpcclient"
	clientobservability "github.com/eannchen/go-backend-architecture/internal/infra/grpcclient/interceptor/observability"
	clientrequestcontext "github.com/eannchen/go-backend-architecture/internal/infra/grpcclient/interceptor/requestcontext"
	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/util/errutil"
)

type App struct {
	*appruntime.Telemetry
	Config      config.Config
	Client      *infragrpcclient.Client
	diagnostics diagnosticsv1.DiagnosticsServiceClient
	health      healthpb.HealthClient
}

var _ appruntime.Application = (*App)(nil)

type Report struct {
	DiagnosticsHealthy bool   `json:"diagnostics_healthy"`
	StandardCheck      string `json:"standard_check"`
	StandardWatch      string `json:"standard_watch"`
}

// New composes the standalone demonstration client and its process telemetry.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	telemetry, err := appruntime.NewTelemetry(ctx, cfg, cfg.GRPCClient.ServiceName)
	if err != nil {
		return nil, err
	}
	app, err := newApp(
		cfg.GRPCClient,
		telemetry.Observability.Tracer(),
		telemetry.Logger,
		telemetry.Observability.Meter(),
	)
	if err != nil {
		return nil, errutil.Join(err, errutil.Step("shutdown telemetry after gRPC client init failure", telemetry.Shutdown(ctx)))
	}
	app.Telemetry = telemetry
	app.Config = cfg
	return app, nil
}

func newApp(cfg config.GRPCClientConfig, tracer observability.Tracer, log logger.Logger, meter observability.Meter, options ...infragrpcclient.Option) (*App, error) {
	transportCredentials, err := buildTransportCredentials(cfg.TLS)
	if err != nil {
		return nil, err
	}
	requestContext, err := clientrequestcontext.New(clientrequestcontext.Config{
		Timeout:              cfg.RequestTimeout,
		RequestIDMetadataKey: cfg.RequestIDMetadataKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create demo request-context interceptors: %w", err)
	}
	requestObservability := clientobservability.New(
		clientobservability.Config{
			DependencyName: cfg.DependencyName,
			Target:         cfg.Target,
		},
		clientobservability.WithTracing(tracer, clientobservability.TraceConfig{
			PropagateContext: cfg.TracePropagation,
		}),
		clientobservability.WithMetrics(meter),
		clientobservability.WithCompletionLog(log, demoLogPolicy),
	)
	options = append([]infragrpcclient.Option{
		infragrpcclient.WithUnaryInterceptors(requestContext.Unary(), requestObservability.Unary()),
		infragrpcclient.WithStreamInterceptors(requestContext.Stream(), requestObservability.Stream()),
	}, options...)
	client, err := infragrpcclient.New(infragrpcclient.Config{
		Target:              cfg.Target,
		MaxRecvMessageBytes: cfg.MaxRecvMessageBytes,
		MaxSendMessageBytes: cfg.MaxSendMessageBytes,
	}, transportCredentials, options...)
	if err != nil {
		return nil, err
	}
	return &App{
		Client:      client,
		diagnostics: diagnosticsv1.NewDiagnosticsServiceClient(client.Connection()),
		health:      healthpb.NewHealthClient(client.Connection()),
	}, nil
}

// Run performs the unary and server-streaming demonstration calls.
func (a *App) Run(ctx context.Context) (Report, error) {
	diagnosticsResponse, err := a.diagnostics.GetHealth(ctx, &diagnosticsv1.GetHealthRequest{
		Mode: diagnosticsv1.HealthCheckMode_HEALTH_CHECK_MODE_READY,
	})
	if err != nil {
		return Report{}, fmt.Errorf("get custom diagnostics health: %w", err)
	}
	checkResponse, err := a.health.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return Report{}, fmt.Errorf("check standard health: %w", err)
	}

	watchCtx, cancelWatch := context.WithCancel(ctx)
	watch, err := a.health.Watch(watchCtx, &healthpb.HealthCheckRequest{})
	if err != nil {
		cancelWatch()
		return Report{}, fmt.Errorf("start standard health watch: %w", err)
	}
	watchResponse, err := watch.Recv()
	if err != nil {
		cancelWatch()
		return Report{}, fmt.Errorf("receive standard health watch: %w", err)
	}
	// Health.Watch is long-lived. Cancel after its initial state, then receive the
	// terminal status so stream interceptors can finish their lifecycle.
	cancelWatch()
	_, terminalErr := watch.Recv()
	if terminalErr != nil && !errors.Is(terminalErr, context.Canceled) && status.Code(terminalErr) != codes.Canceled {
		return Report{}, fmt.Errorf("finish standard health watch: %w", terminalErr)
	}

	return Report{
		DiagnosticsHealthy: diagnosticsResponse.GetHealthy(),
		StandardCheck:      checkResponse.GetStatus().String(),
		StandardWatch:      watchResponse.GetStatus().String(),
	}, nil
}

// Close releases the shared client connection.
func (a *App) Close() error {
	if a == nil {
		return nil
	}
	return a.Client.Close()
}

// Shutdown closes the connection and flushes the standalone process telemetry.
func (a *App) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return errors.Join(a.Close(), a.Telemetry.Shutdown(ctx))
}

func demoLogPolicy(outcome clientobservability.LogOutcome) (logger.Severity, bool) {
	// The demo deliberately cancels Health.Watch after its first response, so that
	// terminal status is an expected lifecycle event rather than a dependency fault.
	if outcome.GRPCStatusCode == codes.OK || outcome.GRPCStatusCode == codes.Canceled {
		return logger.SeverityInfo, true
	}
	return logger.SeverityError, true
}

func buildTransportCredentials(cfg config.GRPCClientTLSConfig) (credentials.TransportCredentials, error) {
	if !cfg.Enabled {
		return insecure.NewCredentials(), nil
	}
	tlsCfg, err := tlsconfig.LoadClient(tlsconfig.ClientConfig{
		ServerName:     cfg.ServerName,
		ServerCAFile:   cfg.ServerCAFile,
		ClientCertFile: cfg.ClientCertFile,
		ClientKeyFile:  cfg.ClientKeyFile,
	})
	if err != nil {
		return nil, fmt.Errorf("load gRPC client TLS configuration: %w", err)
	}
	return credentials.NewTLS(tlsCfg), nil
}
