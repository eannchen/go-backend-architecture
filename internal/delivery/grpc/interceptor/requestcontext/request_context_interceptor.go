package requestcontext

import (
	"context"
	"fmt"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/util/grpcmetadata"
)

// Config controls server deadlines and request-ID interoperability policy.
type Config struct {
	Timeout   time.Duration
	RequestID RequestIDConfig
}

// RequestIDConfig separates accepting and returning an optional request ID.
type RequestIDConfig struct {
	// IncomingMetadataKey accepts a caller's correlation ID. Empty disables it.
	IncomingMetadataKey string
	// ResponseMetadataKey returns an accepted ID to the caller. Empty disables it.
	ResponseMetadataKey string
	// RejectInvalid rejects malformed incoming IDs. The default replaces them
	// so optional correlation metadata cannot prevent valid business work.
	RejectInvalid bool
}

// Interceptor enriches RPC contexts with correlation and deadline metadata.
type Interceptor struct {
	config    Config
	responder grpcresponse.Responder
}

// New validates metadata policy and creates request-context interceptors.
func New(config Config, responder grpcresponse.Responder) (*Interceptor, error) {
	var err error
	config.RequestID.IncomingMetadataKey, err = grpcmetadata.NormalizeKey(config.RequestID.IncomingMetadataKey)
	if err != nil {
		return nil, fmt.Errorf("normalize incoming request ID metadata key: %w", err)
	}
	config.RequestID.ResponseMetadataKey, err = grpcmetadata.NormalizeKey(config.RequestID.ResponseMetadataKey)
	if err != nil {
		return nil, fmt.Errorf("normalize response request ID metadata key: %w", err)
	}
	if responder == nil {
		responder = grpcresponse.NewResponder()
	}
	return &Interceptor{config: config, responder: responder}, nil
}

func (i *Interceptor) Unary() googlegrpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *googlegrpc.UnaryServerInfo,
		handler googlegrpc.UnaryHandler,
	) (any, error) {
		requestCtx, requestID, err := i.enrich(ctx)
		if err != nil {
			return nil, err
		}
		if key := i.config.RequestID.ResponseMetadataKey; key != "" && requestID != "" {
			if err := googlegrpc.SetHeader(requestCtx, metadata.Pairs(key, requestID)); err != nil {
				return nil, i.responder.Error(err, codes.Internal, "internal server error")
			}
		}

		if i.config.Timeout > 0 {
			var cancel context.CancelFunc
			requestCtx, cancel = context.WithTimeout(requestCtx, i.config.Timeout)
			defer cancel()
		}

		return handler(requestCtx, req)
	}
}

func (i *Interceptor) Stream() googlegrpc.StreamServerInterceptor {
	return func(
		srv any,
		stream googlegrpc.ServerStream,
		_ *googlegrpc.StreamServerInfo,
		handler googlegrpc.StreamHandler,
	) error {
		requestCtx, requestID, err := i.enrich(stream.Context())
		if err != nil {
			return err
		}
		if key := i.config.RequestID.ResponseMetadataKey; key != "" && requestID != "" {
			if err := stream.SetHeader(metadata.Pairs(key, requestID)); err != nil {
				return i.responder.Error(err, codes.Internal, "internal server error")
			}
		}

		return handler(srv, &contextServerStream{ServerStream: stream, ctx: requestCtx})
	}
}

func (i *Interceptor) enrich(ctx context.Context) (context.Context, string, error) {
	requestID, err := incomingRequestID(ctx, i.config.RequestID.IncomingMetadataKey)
	if err != nil {
		if i.config.RequestID.RejectInvalid {
			message := fmt.Sprintf("%s must be one value of 1-128 characters from [a-zA-Z0-9._-]", i.config.RequestID.IncomingMetadataKey)
			return ctx, "", i.responder.Error(err, codes.InvalidArgument, message)
		}
		return ctx, "", nil
	}
	if requestID == "" {
		return ctx, "", nil
	}
	return observability.WithRequestID(ctx, requestID), requestID, nil
}

func incomingRequestID(ctx context.Context, metadataKey string) (string, error) {
	if metadataKey == "" {
		return "", nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", nil
	}
	values := md.Get(metadataKey)
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		return "", nil
	}
	if len(values) != 1 || !observability.IsValidRequestID(values[0]) {
		return "", fmt.Errorf("invalid %s metadata: %q", metadataKey, values)
	}
	return values[0], nil
}

type contextServerStream struct {
	googlegrpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }
