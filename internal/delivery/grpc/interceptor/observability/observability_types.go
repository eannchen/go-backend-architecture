package observability

import (
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/observability/grpcsemconv"
)

type rpcInfo struct {
	// method is the normalized, fully-qualified gRPC method without the wire path's leading slash.
	method string
	// callType distinguishes unary and streaming lifecycles; it is a template field, not an OTel field.
	callType string
}

func newRPCInfo(fullMethod, callType string) rpcInfo {
	return rpcInfo{
		method:   grpcsemconv.NormalizeMethod(fullMethod),
		callType: callType,
	}
}

func (i rpcInfo) spanStartFields() appobservability.Fields {
	return appobservability.FromPairs(
		keyRPCSystemName, "grpc",
		keyRPCMethod, i.method,
		keyApplicationRPCCallType, i.callType,
	)
}

type rpcOutcome struct {
	// rpc contains immutable transport facts captured before the service runs.
	rpc rpcInfo
	// duration measures service execution and is used by logs and aggregate metrics.
	duration time.Duration
	// responseStatusCode is the native gRPC status resolved from the returned error.
	responseStatusCode codes.Code
	// handlerError is the exact error returned to gRPC and used to finish the span.
	handlerError error
	// applicationError contains responder-owned application error metadata, when available.
	applicationError applicationErrorInfo
}

func newRPCOutcome(rpc rpcInfo, duration time.Duration, handlerErr error) rpcOutcome {
	rpcStatus := status.Code(handlerErr)
	return rpcOutcome{
		rpc:                rpc,
		duration:           duration,
		responseStatusCode: rpcStatus,
		handlerError:       handlerErr,
		applicationError:   inspectApplicationError(handlerErr),
	}
}

func (o rpcOutcome) responseStatusName() string {
	return grpcsemconv.StatusName(o.responseStatusCode)
}

// errorType follows the OTel gRPC server rule: only statuses classified as
// server failures populate error.type. Client-caused statuses such as
// INVALID_ARGUMENT still describe the RPC outcome without failing the server span.
func (o rpcOutcome) errorType() string {
	return grpcsemconv.ServerErrorType(o.responseStatusCode)
}

func streamType(info *googlegrpc.StreamServerInfo) string {
	switch {
	case info.IsClientStream && info.IsServerStream:
		return "bidi_stream"
	case info.IsClientStream:
		return "client_stream"
	default:
		return "server_stream"
	}
}
