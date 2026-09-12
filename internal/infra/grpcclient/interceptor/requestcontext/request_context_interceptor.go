package requestcontext

import (
	"context"
	"fmt"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/util/grpcmetadata"
)

// Config controls default deadlines and optional request-ID propagation.
type Config struct {
	Timeout              time.Duration
	RequestIDMetadataKey string
}

// Interceptor applies outbound request identity and default deadlines.
type Interceptor struct {
	config Config
}

// New validates metadata policy and creates outbound request-context interceptors.
func New(config Config) (*Interceptor, error) {
	metadataKey, err := grpcmetadata.NormalizeKey(config.RequestIDMetadataKey)
	if err != nil {
		return nil, fmt.Errorf("normalize request ID metadata key: %w", err)
	}
	config.RequestIDMetadataKey = metadataKey
	return &Interceptor{config: config}, nil
}

// Unary applies request context to unary RPCs.
func (i *Interceptor) Unary() googlegrpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, connection *googlegrpc.ClientConn, invoker googlegrpc.UnaryInvoker, opts ...googlegrpc.CallOption) error {
		callCtx, cancel := i.prepare(ctx)
		defer cancel()
		return invoker(callCtx, method, req, reply, connection, opts...)
	}
}

// Stream applies request context to streaming RPCs.
func (i *Interceptor) Stream() googlegrpc.StreamClientInterceptor {
	return func(ctx context.Context, description *googlegrpc.StreamDesc, connection *googlegrpc.ClientConn, method string, streamer googlegrpc.Streamer, opts ...googlegrpc.CallOption) (googlegrpc.ClientStream, error) {
		callCtx, cancel := i.prepare(ctx)
		stream, err := streamer(callCtx, description, connection, method, opts...)
		if err != nil {
			cancel()
			return nil, err
		}
		return &cancelingClientStream{ClientStream: stream, cancel: cancel}, nil
	}
}

func (i *Interceptor) prepare(ctx context.Context) (context.Context, context.CancelFunc) {
	if i.config.RequestIDMetadataKey != "" {
		ctx = i.withRequestIDMetadata(ctx)
	}

	if i.config.Timeout <= 0 {
		return ctx, func() {}
	}
	callCtx, cancel := context.WithTimeout(ctx, i.config.Timeout)
	return callCtx, cancel
}

func (i *Interceptor) withRequestIDMetadata(ctx context.Context) context.Context {
	requestID := observability.RequestIDFromContext(ctx)
	if requestID == "" {
		return ctx
	}

	// Outgoing gRPC metadata lives in the context. Keep metadata already added
	// by the caller or another interceptor rather than replacing it with only a
	// request ID.
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		// MD is a mutable map. Copy it before setting the request ID so the
		// caller's parent context remains reusable and unchanged.
		md = md.Copy()
	}
	md.Set(i.config.RequestIDMetadataKey, requestID)
	// Attach the updated metadata to a derived context. The gRPC client reads
	// this outgoing metadata when it creates the HTTP/2 request headers.
	return metadata.NewOutgoingContext(ctx, md)
}

type cancelingClientStream struct {
	googlegrpc.ClientStream
	cancel context.CancelFunc
}

func (s *cancelingClientStream) SendMsg(message any) error {
	err := s.ClientStream.SendMsg(message)
	if err != nil {
		s.cancel()
	}
	return err
}

func (s *cancelingClientStream) RecvMsg(message any) error {
	err := s.ClientStream.RecvMsg(message)
	if err != nil {
		s.cancel()
	}
	return err
}
