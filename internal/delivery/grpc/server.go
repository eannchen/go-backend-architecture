package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// ServiceRegistrar registers one or more services with a gRPC server.
type ServiceRegistrar interface {
	RegisterGRPC(googlegrpc.ServiceRegistrar)
}

// ServiceRegistrarFunc adapts a registration function to ServiceRegistrar.
type ServiceRegistrarFunc func(googlegrpc.ServiceRegistrar)

func (f ServiceRegistrarFunc) RegisterGRPC(registrar googlegrpc.ServiceRegistrar) {
	if f != nil {
		f(registrar)
	}
}

type ServerConfig struct {
	Address             string
	ReflectionEnabled   bool
	MaxRecvMessageBytes int
	MaxSendMessageBytes int
}

type Server struct {
	logger     logger.Logger
	listener   net.Listener
	grpcServer *googlegrpc.Server
}

func NewServer(
	cfg ServerConfig,
	log logger.Logger,
	unaryInterceptors []googlegrpc.UnaryServerInterceptor,
	streamInterceptors []googlegrpc.StreamServerInterceptor,
	registrars ...ServiceRegistrar,
) (*Server, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("gRPC server address must not be empty")
	}
	if cfg.MaxRecvMessageBytes < 0 || cfg.MaxSendMessageBytes < 0 {
		return nil, fmt.Errorf("gRPC message limits must not be negative")
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("listen for gRPC connections: %w", err)
	}
	return newServer(cfg, log, listener, unaryInterceptors, streamInterceptors, registrars...), nil
}

func newServer(
	cfg ServerConfig,
	log logger.Logger,
	listener net.Listener,
	unaryInterceptors []googlegrpc.UnaryServerInterceptor,
	streamInterceptors []googlegrpc.StreamServerInterceptor,
	registrars ...ServiceRegistrar,
) *Server {
	if log == nil {
		log = logger.NoopLogger{}
	}

	options := make([]googlegrpc.ServerOption, 0, 4)
	if cfg.MaxRecvMessageBytes > 0 {
		options = append(options, googlegrpc.MaxRecvMsgSize(cfg.MaxRecvMessageBytes))
	}
	if cfg.MaxSendMessageBytes > 0 {
		options = append(options, googlegrpc.MaxSendMsgSize(cfg.MaxSendMessageBytes))
	}
	if len(unaryInterceptors) > 0 {
		options = append(options, googlegrpc.ChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		options = append(options, googlegrpc.ChainStreamInterceptor(streamInterceptors...))
	}

	grpcServer := googlegrpc.NewServer(options...)
	for _, registrar := range registrars {
		if registrar != nil {
			registrar.RegisterGRPC(grpcServer)
		}
	}
	if cfg.ReflectionEnabled {
		reflection.Register(grpcServer)
	}

	return &Server{
		logger:     log,
		listener:   listener,
		grpcServer: grpcServer,
	}
}

func (s *Server) Start() error {
	s.logger.Info(context.Background(), "gRPC server starting", logger.FromPairs("address", s.listener.Addr().String()))

	if err := s.grpcServer.Serve(s.listener); err != nil && !errors.Is(err, googlegrpc.ErrServerStopped) {
		return fmt.Errorf("serve gRPC: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	startedAt := time.Now()
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info(ctx, "gRPC server shutdown complete", logger.FromPairs("duration_ms", time.Since(startedAt).Milliseconds()))
		return nil
	case <-ctx.Done():
		s.grpcServer.Stop()
		<-done
		return fmt.Errorf("gRPC server graceful shutdown: %w", ctx.Err())
	}
}

func (s *Server) Address() net.Addr {
	return s.listener.Addr()
}
