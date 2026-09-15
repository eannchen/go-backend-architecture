package grpcapi

import (
	"context"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
	"github.com/eannchen/go-backend-architecture/internal/usecase/health/healthtest"
)

func TestBuildServerWiresServicesRequestContextAndRecovery(t *testing.T) {
	const requestIDKey = "correlation-id"

	requestContexts := make(chan context.Context, 2)
	panicOnCheck := false
	healthUsecase := &healthtest.Usecase{
		CheckFunc: func(ctx context.Context, _ usecasehealth.CheckMode) (usecasehealth.Result, error) {
			requestContexts <- ctx
			if panicOnCheck {
				panic("health check panic")
			}
			return usecasehealth.Result{}, nil
		},
	}
	wiring := newWiring(config.Config{GRPC: config.GRPCConfig{
		Address:               "127.0.0.1:0",
		HealthRefreshInterval: time.Hour,
		RequestTimeout:        time.Second,
		MaxRecvMessageBytes:   1 << 20,
		MaxSendMessageBytes:   1 << 20,
		RequestID: config.RequestIDConfig{
			IncomingKey: requestIDKey,
			ResponseKey: requestIDKey,
		},
	}}, logger.NoopLogger{}, observability.NoopTracer{}, observability.NoopMeter{})

	components, err := wiring.buildServer(healthUsecase)
	if err != nil {
		t.Fatalf("buildServer() error = %v", err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- components.server.Start() }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := components.reporter.Shutdown(shutdownCtx); err != nil {
			t.Errorf("shutdown health reporter: %v", err)
		}
		if err := components.server.Shutdown(shutdownCtx); err != nil {
			t.Errorf("shutdown gRPC server: %v", err)
		}
		if err := <-serveErr; err != nil {
			t.Errorf("serve gRPC: %v", err)
		}
	})

	conn, err := googlegrpc.NewClient(
		components.server.Address().String(),
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create gRPC client: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close client connection: %v", err)
		}
	})

	callCtx, cancelCall := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelCall()
	callCtx = metadata.AppendToOutgoingContext(callCtx, requestIDKey, "request-123")
	var responseHeader metadata.MD
	_, err = diagnosticsv1.NewDiagnosticsServiceClient(conn).GetHealth(
		callCtx,
		&diagnosticsv1.GetHealthRequest{},
		googlegrpc.Header(&responseHeader),
	)
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	requestCtx := <-requestContexts
	if got := observability.RequestIDFromContext(requestCtx); got != "request-123" {
		t.Fatalf("usecase request ID = %q, want request-123", got)
	}
	deadline, ok := requestCtx.Deadline()
	if !ok || time.Until(deadline) > time.Second {
		t.Fatalf("usecase deadline = %v, %t; want server deadline within one second", deadline, ok)
	}
	if got := responseHeader.Get(requestIDKey); len(got) != 1 || got[0] != "request-123" {
		t.Fatalf("response request ID = %v, want request-123", got)
	}

	healthResponse, err := healthpb.NewHealthClient(conn).Check(callCtx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("standard health Check() error = %v", err)
	}
	if healthResponse.GetStatus() != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("standard health status = %s, want NOT_SERVING", healthResponse.GetStatus())
	}

	panicOnCheck = true
	_, err = diagnosticsv1.NewDiagnosticsServiceClient(conn).GetHealth(callCtx, &diagnosticsv1.GetHealthRequest{})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("panic status = %s, want Internal", got)
	}
	<-requestContexts
}
