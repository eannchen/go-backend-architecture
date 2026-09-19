package grpcclient

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpcstandardhealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig"
	"github.com/eannchen/go-backend-architecture/internal/infra/security/tlsconfig/tlsconfigtest"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestClientDoesNotApplyUnselectedMetadataPolicies(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	var receivedMetadata metadata.MD
	server := googlegrpc.NewServer(googlegrpc.UnaryInterceptor(func(ctx context.Context, request any, info *googlegrpc.UnaryServerInfo, handler googlegrpc.UnaryHandler) (any, error) {
		receivedMetadata, _ = metadata.FromIncomingContext(ctx)
		return handler(ctx, request)
	}))
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

	client, err := New(
		validConfig(),
		insecure.NewCredentials(),
		WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctx := observability.WithRequestID(context.Background(), "request-123")
	response, err := healthpb.NewHealthClient(client.Connection()).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health status = %v, want SERVING", response.GetStatus())
	}
	if got := receivedMetadata.Get("x-request-id"); len(got) != 0 {
		t.Fatalf("x-request-id metadata = %v, want none", got)
	}
	if got := receivedMetadata.Get("traceparent"); len(got) != 0 {
		t.Fatalf("traceparent metadata = %v, want none", got)
	}
}

func TestClientConnectsWithTLSAndMutualTLS(t *testing.T) {
	tests := []struct {
		name      string
		mutualTLS bool
	}{
		{name: "TLS"},
		{name: "mutual TLS", mutualTLS: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authority := tlsconfigtest.NewCertificateAuthority(t)
			serverCertificateFile, serverKeyFile := tlsconfigtest.WriteCertificateFiles(t, "server", authority.IssueServerCertificate(t, "localhost"))
			serverConfig := tlsconfig.ServerConfig{
				ServerCertFile:    serverCertificateFile,
				ServerKeyFile:     serverKeyFile,
				RequireClientCert: tt.mutualTLS,
			}
			clientConfig := tlsconfig.ClientConfig{
				ServerName:   "localhost",
				ServerCAFile: authority.WriteCAFile(t),
			}
			if tt.mutualTLS {
				serverConfig.ClientCAFile = authority.WriteCAFile(t)
				clientConfig.ClientCertFile, clientConfig.ClientKeyFile = tlsconfigtest.WriteCertificateFiles(t, "client", authority.IssueClientCertificate(t, "test client"))
			}

			serverTLS, err := tlsconfig.LoadServer(serverConfig)
			if err != nil {
				t.Fatalf("LoadServer() error = %v", err)
			}
			clientTLS, err := tlsconfig.LoadClient(clientConfig)
			if err != nil {
				t.Fatalf("LoadClient() error = %v", err)
			}

			listener := bufconn.Listen(1 << 20)
			server := googlegrpc.NewServer(googlegrpc.Creds(credentials.NewTLS(serverTLS)))
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

			client, err := New(validConfig(), credentials.NewTLS(clientTLS), WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return listener.DialContext(ctx)
			}))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			t.Cleanup(func() { _ = client.Close() })

			response, err := healthpb.NewHealthClient(client.Connection()).Check(context.Background(), &healthpb.HealthCheckRequest{})
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
				t.Fatalf("health status = %v, want SERVING", response.GetStatus())
			}
		})
	}
}

func TestClientReportsLazyConnectionFailureWithinCallerDeadline(t *testing.T) {
	wantErr := errors.New("dial failed")
	client, err := New(validConfig(), insecure.NewCredentials(), WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return nil, wantErr
	}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = healthpb.NewHealthClient(client.Connection()).Check(ctx, &healthpb.HealthCheckRequest{})
	if err == nil {
		t.Fatal("Check() error = nil, want connection failure")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("connection failure took %s, want bounded by default deadline", elapsed)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		credentials bool
		wantErr     string
	}{
		{name: "empty target", cfg: Config{MaxRecvMessageBytes: 1, MaxSendMessageBytes: 1}, credentials: true, wantErr: "target is required"},
		{name: "non-positive limits", cfg: Config{Target: "localhost:9090"}, credentials: true, wantErr: "message limits must be > 0"},
		{name: "missing credentials", cfg: validConfig(), wantErr: "transport credentials are required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var credentials = insecure.NewCredentials()
			if !tt.credentials {
				credentials = nil
			}
			_, err := New(tt.cfg, credentials)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("New() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func validConfig() Config {
	return Config{Target: "passthrough:///bufconn", MaxRecvMessageBytes: 1 << 20, MaxSendMessageBytes: 1 << 20}
}
