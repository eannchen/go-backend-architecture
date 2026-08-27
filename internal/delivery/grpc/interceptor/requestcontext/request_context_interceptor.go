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
)

const RequestIDMetadataKey = "x-request-id"

// Interceptor enriches RPC contexts with correlation and deadline metadata.
type Interceptor struct {
	timeout   time.Duration
	responder grpcresponse.Responder
}

func New(timeout time.Duration, responder grpcresponse.Responder) *Interceptor {
	if responder == nil {
		responder = grpcresponse.NewResponder()
	}
	return &Interceptor{timeout: timeout, responder: responder}
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
		if err := googlegrpc.SetHeader(requestCtx, metadata.Pairs(RequestIDMetadataKey, requestID)); err != nil {
			return nil, i.responder.Error(err, codes.Internal, "internal server error")
		}

		if i.timeout > 0 {
			var cancel context.CancelFunc
			requestCtx, cancel = context.WithTimeout(requestCtx, i.timeout)
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
		if err := stream.SetHeader(metadata.Pairs(RequestIDMetadataKey, requestID)); err != nil {
			return i.responder.Error(err, codes.Internal, "internal server error")
		}

		return handler(srv, &contextServerStream{ServerStream: stream, ctx: requestCtx})
	}
}

func (i *Interceptor) enrich(ctx context.Context) (context.Context, string, error) {
	requestID, err := incomingRequestID(ctx)
	if err != nil {
		return ctx, "", i.responder.Error(err, codes.InvalidArgument, "x-request-id must be one value of 1-128 characters from [a-zA-Z0-9._-]")
	}
	if requestID == "" {
		requestID, err = observability.GenerateRequestID()
		if err != nil {
			return ctx, "", i.responder.Error(err, codes.Internal, "internal server error")
		}
	}
	return observability.WithRequestID(ctx, requestID), requestID, nil
}

func incomingRequestID(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", nil
	}
	values := md.Get(RequestIDMetadataKey)
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		return "", nil
	}
	if len(values) != 1 || !observability.IsValidRequestID(values[0]) {
		return "", fmt.Errorf("invalid %s metadata: %q", RequestIDMetadataKey, values)
	}
	return values[0], nil
}

type contextServerStream struct {
	googlegrpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }
