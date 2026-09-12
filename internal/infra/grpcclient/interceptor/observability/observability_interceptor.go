package observability

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	googlegrpc "google.golang.org/grpc"

	"github.com/eannchen/go-backend-architecture/internal/logger"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

// Config identifies the dependency and logical endpoint represented by this connection.
type Config struct {
	// DependencyName is an optional stable application name used to group this dependency's telemetry.
	DependencyName string
	// Target is the same logical gRPC target used to create the client connection.
	Target string
}

// Option enables one reusable observability capability for the connection.
type Option func(*Interceptor)

// WithTracing enables local client spans and independently controls propagation.
func WithTracing(tracer appobservability.Tracer, config TraceConfig) Option {
	return func(interceptor *Interceptor) {
		interceptor.tracing = NewTracing(tracer, config)
	}
}

// WithMetrics enables aggregate RPC and stream-lifecycle measurements.
func WithMetrics(meter appobservability.Meter) Option {
	return func(interceptor *Interceptor) {
		interceptor.metrics = NewRequestMetrics(meter)
	}
}

// WithCompletionLog enables completion logs governed by dependency policy.
func WithCompletionLog(log logger.Logger, policy LogPolicy) Option {
	return func(interceptor *Interceptor) {
		interceptor.accessLog = NewAccessLog(log, policy)
	}
}

// Interceptor composes tracing, metrics, and access logging for client RPCs.
type Interceptor struct {
	dependencyName string
	serverAddress  string
	serverPort     int
	tracing        *Tracing
	metrics        *RequestMetrics
	accessLog      *AccessLog
}

// New creates an interceptor from only the capabilities selected by its caller.
func New(config Config, options ...Option) *Interceptor {
	config.DependencyName = strings.TrimSpace(config.DependencyName)
	config.Target = strings.TrimSpace(config.Target)
	serverAddress, serverPort := parseServerEndpoint(config.Target)
	interceptor := &Interceptor{
		dependencyName: config.DependencyName,
		serverAddress:  serverAddress,
		serverPort:     serverPort,
	}
	for _, option := range options {
		option(interceptor)
	}
	return interceptor
}

// Unary instruments unary client calls.
func (i *Interceptor) Unary() googlegrpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, connection *googlegrpc.ClientConn, invoker googlegrpc.UnaryInvoker, opts ...googlegrpc.CallOption) error {
		if !i.enabled() {
			return invoker(ctx, method, req, reply, connection, opts...)
		}
		rpc := newRPCInfo(i.dependencyName, i.serverAddress, i.serverPort, method, "unary")
		var span appobservability.Span
		if i.tracing != nil {
			ctx, span = i.tracing.Start(ctx, rpc)
		}
		started := time.Now()

		err := invoker(ctx, method, req, reply, connection, opts...)
		i.finish(ctx, span, newRPCOutcome(rpc, time.Since(started), err), false)
		return err
	}
}

// Stream instruments stream setup and records the terminal stream outcome once.
func (i *Interceptor) Stream() googlegrpc.StreamClientInterceptor {
	return func(ctx context.Context, description *googlegrpc.StreamDesc, connection *googlegrpc.ClientConn, method string, streamer googlegrpc.Streamer, opts ...googlegrpc.CallOption) (googlegrpc.ClientStream, error) {
		if !i.enabled() {
			return streamer(ctx, description, connection, method, opts...)
		}
		rpc := newRPCInfo(i.dependencyName, i.serverAddress, i.serverPort, method, streamType(description))
		var span appobservability.Span
		if i.tracing != nil {
			ctx, span = i.tracing.Start(ctx, rpc)
		}
		started := time.Now()
		if i.metrics != nil {
			i.metrics.StreamStarted(ctx, rpc)
		}

		stream, err := streamer(ctx, description, connection, method, opts...)
		if err != nil {
			i.finish(ctx, span, newRPCOutcome(rpc, time.Since(started), err), true)
			return nil, err
		}
		return &observedClientStream{
			ClientStream: stream,
			finish: func(err error) {
				if errors.Is(err, io.EOF) {
					err = nil
				}
				i.finish(ctx, span, newRPCOutcome(rpc, time.Since(started), err), true)
			},
		}, nil
	}
}

func (i *Interceptor) finish(ctx context.Context, span appobservability.Span, outcome rpcOutcome, stream bool) {
	if i.metrics != nil {
		if stream {
			i.metrics.StreamFinished(ctx, outcome)
		} else {
			i.metrics.Record(ctx, outcome)
		}
	}
	if i.accessLog != nil {
		i.accessLog.Record(ctx, outcome)
	}
	if i.tracing != nil {
		i.tracing.Finish(span, outcome)
	}
}

func (i *Interceptor) enabled() bool {
	return i.tracing != nil || i.metrics != nil || i.accessLog != nil
}

type observedClientStream struct {
	googlegrpc.ClientStream
	once   sync.Once
	finish func(error)
}

func (s *observedClientStream) SendMsg(message any) error {
	err := s.ClientStream.SendMsg(message)
	if err != nil {
		s.once.Do(func() { s.finish(err) })
	}
	return err
}

func (s *observedClientStream) RecvMsg(message any) error {
	err := s.ClientStream.RecvMsg(message)
	if err != nil {
		s.once.Do(func() { s.finish(err) })
	}
	return err
}
