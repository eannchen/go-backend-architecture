package grpcclient

import (
	"context"
	"net"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	grpcstandardhealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	infragrpcclient "github.com/eannchen/go-backend-architecture/internal/infra/grpcclient"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/observabilitytest"
)

func TestAppRunsUnaryAndStreamingHealthDemo(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := googlegrpc.NewServer()
	receivedMetadata := make(chan metadata.MD, 1)
	diagnosticsv1.RegisterDiagnosticsServiceServer(server, demoDiagnosticsServer{receivedMetadata: receivedMetadata})
	healthServer := grpcstandardhealth.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
		<-serveErr
	})

	span := &observabilitytest.Span{
		SetAttributesFunc: func(...observability.Fields) {},
		FinishFunc:        func(error, ...string) {},
	}
	tracer := &observabilitytest.Tracer{
		StartClientFunc: func(ctx context.Context, _, _ string, _ ...observability.Fields) (context.Context, observability.Span) {
			return ctx, span
		},
		InjectFunc: func(_ context.Context, carrier observability.TextMapCarrier) {
			carrier.Set("traceparent", "propagated-trace-context")
		},
	}
	app, err := newApp(config.GRPCClientConfig{
		DependencyName:       "grpcapi-demo",
		Target:               "passthrough:///bufconn",
		RequestTimeout:       time.Second,
		MaxRecvMessageBytes:  1 << 20,
		MaxSendMessageBytes:  1 << 20,
		RequestIDMetadataKey: "x-request-id",
		TracePropagation:     true,
	}, tracer, nil, nil, infragrpcclient.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}))
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	report, err := app.Run(observability.WithRequestID(context.Background(), "request-123"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.DiagnosticsHealthy || report.StandardCheck != "SERVING" || report.StandardWatch != "SERVING" {
		t.Fatalf("report = %+v", report)
	}
	md := <-receivedMetadata
	if got := md.Get("x-request-id"); len(got) != 1 || got[0] != "request-123" {
		t.Fatalf("x-request-id metadata = %v", got)
	}
	if got := md.Get("traceparent"); len(got) != 1 || got[0] != "propagated-trace-context" {
		t.Fatalf("traceparent metadata = %v", got)
	}
	if tracer.StartClientCalls != 3 || tracer.InjectCalls != 3 {
		t.Fatalf("tracing calls = start:%d inject:%d, want 3 each", tracer.StartClientCalls, tracer.InjectCalls)
	}
}

type demoDiagnosticsServer struct {
	diagnosticsv1.UnimplementedDiagnosticsServiceServer
	receivedMetadata chan<- metadata.MD
}

func (s demoDiagnosticsServer) GetHealth(ctx context.Context, _ *diagnosticsv1.GetHealthRequest) (*diagnosticsv1.GetHealthResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.receivedMetadata <- md.Copy()
	return &diagnosticsv1.GetHealthResponse{Healthy: true}, nil
}
