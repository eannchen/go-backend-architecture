package grpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstandardhealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"

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

func TestNewServerRejectsNegativeMessageLimits(t *testing.T) {
	tests := []ServerConfig{
		{Address: "127.0.0.1:0", MaxRecvMessageBytes: -1},
		{Address: "127.0.0.1:0", MaxSendMessageBytes: -1},
	}
	for _, cfg := range tests {
		if _, err := NewServer(cfg, nil, nil, nil); err == nil {
			t.Fatalf("NewServer(%+v) error = nil, want validation error", cfg)
		}
	}
}

func TestServerEnforcesMessageLimits(t *testing.T) {
	tests := []struct {
		name         string
		cfg          ServerConfig
		requestSize  int
		responseSize int
	}{
		{
			name:         "receive limit",
			cfg:          ServerConfig{Address: "bufconn", MaxRecvMessageBytes: 64, MaxSendMessageBytes: 1024},
			requestSize:  256,
			responseSize: 1,
		},
		{
			name:         "send limit",
			cfg:          ServerConfig{Address: "bufconn", MaxRecvMessageBytes: 1024, MaxSendMessageBytes: 64},
			requestSize:  1,
			responseSize: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener := bufconn.Listen(1 << 20)
			service := &payloadServiceServer{responseSize: tt.responseSize}
			server := newServer(
				tt.cfg,
				nil,
				listener,
				nil,
				nil,
				ServiceRegistrarFunc(func(registrar googlegrpc.ServiceRegistrar) {
					registrar.RegisterService(&payloadServiceDescription, service)
				}),
			)
			serveErr := make(chan error, 1)
			go func() { serveErr <- server.Start() }()

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

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			response := new(wrapperspb.BytesValue)
			err = conn.Invoke(ctx, "/test.PayloadService/Echo", wrapperspb.Bytes(make([]byte, tt.requestSize)), response)
			if got := status.Code(err); got != codes.ResourceExhausted {
				t.Fatalf("status = %v, want ResourceExhausted", got)
			}

			if err := conn.Close(); err != nil {
				t.Errorf("close client: %v", err)
			}
			if err := server.Shutdown(ctx); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
			if err := <-serveErr; err != nil {
				t.Fatalf("Start() error = %v", err)
			}
		})
	}
}

type payloadService interface {
	Echo(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error)
}

type payloadServiceServer struct{ responseSize int }

func (s *payloadServiceServer) Echo(context.Context, *wrapperspb.BytesValue) (*wrapperspb.BytesValue, error) {
	return wrapperspb.Bytes(make([]byte, s.responseSize)), nil
}

func payloadEchoHandler(
	srv any,
	ctx context.Context,
	decode func(any) error,
	interceptor googlegrpc.UnaryServerInterceptor,
) (any, error) {
	request := new(wrapperspb.BytesValue)
	if err := decode(request); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(payloadService).Echo(ctx, request)
	}
	info := &googlegrpc.UnaryServerInfo{Server: srv, FullMethod: "/test.PayloadService/Echo"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(payloadService).Echo(ctx, req.(*wrapperspb.BytesValue))
	}
	return interceptor(ctx, request, info, handler)
}

var payloadServiceDescription = googlegrpc.ServiceDesc{
	ServiceName: "test.PayloadService",
	HandlerType: (*payloadService)(nil),
	Methods: []googlegrpc.MethodDesc{
		{MethodName: "Echo", Handler: payloadEchoHandler},
	},
}
