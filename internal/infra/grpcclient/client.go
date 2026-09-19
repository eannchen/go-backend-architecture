package grpcclient

import (
	"context"
	"fmt"
	"net"
	"strings"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Config controls one reusable outbound gRPC connection.
type Config struct {
	Target              string
	MaxRecvMessageBytes int
	MaxSendMessageBytes int
}

// Client owns a shared connection and its transport-wide policies.
type Client struct {
	connection *googlegrpc.ClientConn
}

type clientOptions struct {
	dialer             func(context.Context, string) (net.Conn, error)
	unaryInterceptors  []googlegrpc.UnaryClientInterceptor
	streamInterceptors []googlegrpc.StreamClientInterceptor
}

// Option customizes connection construction without changing production defaults.
type Option func(*clientOptions)

// WithContextDialer replaces network dialing, primarily for in-memory tests.
func WithContextDialer(dialer func(context.Context, string) (net.Conn, error)) Option {
	return func(options *clientOptions) { options.dialer = dialer }
}

// WithUnaryInterceptors installs policies explicitly selected for this connection.
func WithUnaryInterceptors(interceptors ...googlegrpc.UnaryClientInterceptor) Option {
	return func(options *clientOptions) {
		options.unaryInterceptors = append(options.unaryInterceptors, interceptors...)
	}
}

// WithStreamInterceptors installs policies explicitly selected for this connection.
func WithStreamInterceptors(interceptors ...googlegrpc.StreamClientInterceptor) Option {
	return func(options *clientOptions) {
		options.streamInterceptors = append(options.streamInterceptors, interceptors...)
	}
}

// New creates a lazy outbound gRPC connection with explicitly selected policies.
func New(cfg Config, transportCredentials credentials.TransportCredentials, options ...Option) (*Client, error) {
	cfg.Target = strings.TrimSpace(cfg.Target)
	if cfg.Target == "" {
		return nil, fmt.Errorf("gRPC client target is required")
	}
	if cfg.MaxRecvMessageBytes <= 0 || cfg.MaxSendMessageBytes <= 0 {
		return nil, fmt.Errorf("gRPC client receive and send message limits must be > 0")
	}
	if transportCredentials == nil {
		return nil, fmt.Errorf("gRPC client transport credentials are required")
	}

	var resolvedOptions clientOptions
	for _, option := range options {
		option(&resolvedOptions)
	}
	dialOptions := []googlegrpc.DialOption{
		googlegrpc.WithTransportCredentials(transportCredentials),
		googlegrpc.WithDefaultCallOptions(
			googlegrpc.MaxCallRecvMsgSize(cfg.MaxRecvMessageBytes),
			googlegrpc.MaxCallSendMsgSize(cfg.MaxSendMessageBytes),
		),
	}
	if len(resolvedOptions.unaryInterceptors) > 0 {
		dialOptions = append(dialOptions, googlegrpc.WithChainUnaryInterceptor(resolvedOptions.unaryInterceptors...))
	}
	if len(resolvedOptions.streamInterceptors) > 0 {
		dialOptions = append(dialOptions, googlegrpc.WithChainStreamInterceptor(resolvedOptions.streamInterceptors...))
	}
	if resolvedOptions.dialer != nil {
		dialOptions = append(dialOptions, googlegrpc.WithContextDialer(resolvedOptions.dialer))
	}

	connection, err := googlegrpc.NewClient(cfg.Target, dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("create gRPC client connection: %w", err)
	}
	return &Client{connection: connection}, nil
}

// Connection returns the shared connection used to construct generated clients.
func (c *Client) Connection() *googlegrpc.ClientConn {
	return c.connection
}

// Close releases connection resources.
func (c *Client) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	if err := c.connection.Close(); err != nil {
		return fmt.Errorf("close gRPC client connection: %w", err)
	}
	return nil
}
