package grpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcstandardhealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

func TestServerRegistersAndServesServices(t *testing.T) {
	standardHealth := grpcstandardhealth.NewServer()
	standardHealth.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	listener := bufconn.Listen(1 << 20)
	server := newServer(
		ServerConfig{Address: "bufconn"},
		&loggertest.Logger{},
		listener,
		nil,
		nil,
		ServiceRegistrarFunc(func(registrar googlegrpc.ServiceRegistrar) {
			healthpb.RegisterHealthServer(registrar, standardHealth)
		}),
	)
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Start()
	}()

	conn, err := googlegrpc.NewClient(
		"passthrough:///bufconn",
		googlegrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health status = %v, want SERVING", response.GetStatus())
	}

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := <-serveErr; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestServerRegistersReflectionWhenEnabled(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := newServer(
		ServerConfig{Address: "bufconn", ReflectionEnabled: true},
		nil,
		listener,
		nil,
		nil,
	)

	services := server.grpcServer.GetServiceInfo()
	if _, ok := services["grpc.reflection.v1.ServerReflection"]; !ok {
		t.Fatal("v1 reflection service was not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
}

func TestServerForcesStopWhenGracefulShutdownExpires(t *testing.T) {
	standardHealth := grpcstandardhealth.NewServer()
	standardHealth.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	listener := bufconn.Listen(1 << 20)
	server := newServer(
		ServerConfig{Address: "bufconn"},
		nil,
		listener,
		nil,
		nil,
		ServiceRegistrarFunc(func(registrar googlegrpc.ServiceRegistrar) {
			healthpb.RegisterHealthServer(registrar, standardHealth)
		}),
	)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Start()
	}()

	conn, err := googlegrpc.NewClient(
		"passthrough:///bufconn",
		googlegrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	}()

	watchCtx, cancelWatch := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelWatch()
	watch, err := healthpb.NewHealthClient(conn).Watch(watchCtx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	if _, err := watch.Recv(); err != nil {
		t.Fatalf("receive initial health status: %v", err)
	}

	shutdownCtx, cancelShutdown := context.WithCancel(context.Background())
	cancelShutdown()
	err = server.Shutdown(shutdownCtx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown() error = %v, want context cancellation", err)
	}
	if err := <-serveErr; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}
