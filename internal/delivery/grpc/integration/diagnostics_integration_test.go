package integration_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcstandardhealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	observabilityinterceptor "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/interceptor/observability"
	recoveryinterceptor "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/interceptor/recovery"
	requestcontextinterceptor "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/interceptor/requestcontext"
	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	diagnosticsservice "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/service/diagnostics"
	grpchealth "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/service/health"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
	"github.com/eannchen/go-backend-architecture/internal/usecase/health/healthtest"
)

const bufferSize = 1 << 20

func TestDetailedAndStandardHealthServicesCoexist(t *testing.T) {
	result := healthyReadyResult()
	var checkErr error
	uc := &healthtest.Usecase{
		CheckFunc: func(context.Context, usecasehealth.CheckMode) (usecasehealth.Result, error) {
			return result, checkErr
		},
	}

	listener := bufconn.Listen(bufferSize)
	responder := grpcresponse.NewResponder()
	requestContext := requestcontextinterceptor.New(time.Second, responder)
	requestObservability := observabilityinterceptor.New(observability.NoopTracer{}, logger.NoopLogger{}, observability.NoopMeter{})
	recovery := recoveryinterceptor.New(logger.NoopLogger{}, responder)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(requestContext.Unary(), requestObservability.Unary(), recovery.Unary()),
		grpc.ChainStreamInterceptor(requestContext.Stream(), requestObservability.Stream(), recovery.Stream()),
	)
	diagnosticsv1.RegisterDiagnosticsServiceServer(server, diagnosticsservice.NewService(uc, responder))
	standardHealth := grpcstandardhealth.NewServer()
	healthpb.RegisterHealthServer(server, standardHealth)
	reporter, err := grpchealth.NewReporter(
		grpchealth.ReporterConfig{RefreshInterval: time.Hour},
		logger.NoopLogger{},
		uc,
		standardHealth,
		diagnosticsv1.DiagnosticsService_ServiceDesc.ServiceName,
	)
	if err != nil {
		t.Fatalf("create standard health reporter: %v", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		server.Stop()
		t.Fatalf("create client connection: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close client connection: %v", err)
		}
		server.Stop()
		if err := listener.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
		if err := <-serveErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("serve: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	diagnosticsClient := diagnosticsv1.NewDiagnosticsServiceClient(conn)
	requestCtx := metadata.AppendToOutgoingContext(ctx, requestcontextinterceptor.RequestIDMetadataKey, "integration-01")
	var responseHeader metadata.MD
	detailed, err := diagnosticsClient.GetHealth(requestCtx, &diagnosticsv1.GetHealthRequest{}, grpc.Header(&responseHeader))
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	if !detailed.GetHealthy() || detailed.GetDatabase().GetName() != "app" {
		t.Fatalf("unexpected detailed health response: %v", detailed)
	}
	if got := responseHeader.Get(requestcontextinterceptor.RequestIDMetadataKey); len(got) != 1 || got[0] != "integration-01" {
		t.Fatalf("response request ID = %v, want integration-01", got)
	}

	standardClient := healthpb.NewHealthClient(conn)
	initial, err := standardClient.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("initial standard Check() error = %v", err)
	}
	if initial.GetStatus() != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("initial status = %v, want NOT_SERVING", initial.GetStatus())
	}

	if err := reporter.Refresh(ctx); err != nil {
		t.Fatalf("healthy Refresh() error = %v", err)
	}
	serving, err := standardClient.Check(ctx, &healthpb.HealthCheckRequest{
		Service: diagnosticsv1.DiagnosticsService_ServiceDesc.ServiceName,
	})
	if err != nil {
		t.Fatalf("serving standard Check() error = %v", err)
	}
	if serving.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("service status = %v, want SERVING", serving.GetStatus())
	}

	watch, err := standardClient.Watch(ctx, &healthpb.HealthCheckRequest{
		Service: diagnosticsv1.DiagnosticsService_ServiceDesc.ServiceName,
	})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	update, err := watch.Recv()
	if err != nil {
		t.Fatalf("receive initial Watch() status: %v", err)
	}
	if update.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("initial watch status = %v, want SERVING", update.GetStatus())
	}

	result.Cache.Status = "down"
	result.KVStore.Status = "skipped"
	checkErr = apperr.New(apperr.CodeUnavailable, "cache readiness failed")
	if err := reporter.Refresh(ctx); err == nil {
		t.Fatal("unhealthy Refresh() error = nil, want readiness error")
	}
	update, err = watch.Recv()
	if err != nil {
		t.Fatalf("receive unhealthy Watch() status: %v", err)
	}
	if update.GetStatus() != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("updated watch status = %v, want NOT_SERVING", update.GetStatus())
	}

	detailed, err = diagnosticsClient.GetHealth(ctx, &diagnosticsv1.GetHealthRequest{})
	if err != nil {
		t.Fatalf("unhealthy GetHealth() error = %v", err)
	}
	if detailed.GetHealthy() || detailed.GetCache().GetStatus() != diagnosticsv1.HealthStatus_HEALTH_STATUS_DOWN {
		t.Fatalf("unexpected unhealthy diagnostics response: %v", detailed)
	}
}

func healthyReadyResult() usecasehealth.Result {
	return usecasehealth.Result{
		Database: usecasehealth.Database{Status: "up", Name: "app", UptimeSeconds: 123},
		Cache:    usecasehealth.Dependency{Status: "up"},
		KVStore:  usecasehealth.Dependency{Status: "up"},
		Vector:   usecasehealth.Dependency{Status: "up"},
	}
}
