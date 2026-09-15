package recovery

import (
	"context"
	"fmt"
	"runtime/debug"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Interceptor converts panics into safe gRPC failures.
type Interceptor struct {
	log       logger.Logger
	responder grpcresponse.Responder
}

func New(log logger.Logger, responder grpcresponse.Responder) *Interceptor {
	if log == nil {
		log = logger.NoopLogger{}
	}
	if responder == nil {
		responder = grpcresponse.NewResponder()
	}
	return &Interceptor{log: log, responder: responder}
}

func (i *Interceptor) Unary() googlegrpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *googlegrpc.UnaryServerInfo,
		handler googlegrpc.UnaryHandler,
	) (response any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				cause := panicError(recovered)
				i.log.Error(ctx, "gRPC panic recovered", cause, logger.FromPairs(
					"rpc.method", info.FullMethod,
					"panic.stack", string(debug.Stack()),
				))
				response = nil
				err = i.responder.Error(cause, codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func (i *Interceptor) Stream() googlegrpc.StreamServerInterceptor {
	return func(
		srv any,
		stream googlegrpc.ServerStream,
		info *googlegrpc.StreamServerInfo,
		handler googlegrpc.StreamHandler,
	) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				cause := panicError(recovered)
				i.log.Error(stream.Context(), "gRPC stream panic recovered", cause, logger.FromPairs(
					"rpc.method", info.FullMethod,
					"panic.stack", string(debug.Stack()),
				))
				err = i.responder.Error(cause, codes.Internal, "internal server error")
			}
		}()
		return handler(srv, stream)
	}
}

func panicError(recovered any) error {
	if err, ok := recovered.(error); ok {
		return fmt.Errorf("panic: %w", err)
	}
	return fmt.Errorf("panic: %v", recovered)
}
