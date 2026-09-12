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
	// dependencyName is the caller-defined stable name for the remote dependency.
	dependencyName string
	// serverAddress is the low-cardinality logical destination derived from the configured target.
	serverAddress string
	// serverPort is the logical destination port, or zero when the target does not expose one safely.
	serverPort int
	// method is the normalized, fully-qualified gRPC method without the wire path's leading slash.
	method string
	// callType distinguishes unary and streaming lifecycles; it is a template field, not an OTel field.
	callType string
}

func newRPCInfo(dependencyName, serverAddress string, serverPort int, fullMethod, callType string) rpcInfo {
	return rpcInfo{
		dependencyName: dependencyName,
		serverAddress:  serverAddress,
		serverPort:     serverPort,
		method:         grpcsemconv.NormalizeMethod(fullMethod),
		callType:       callType,
	}
}

func (i rpcInfo) spanStartFields() appobservability.Fields {
	fields := appobservability.FromPairs(
		keyRPCSystemName, "grpc",
		keyRPCMethod, i.method,
		keyApplicationRPCCallType, i.callType,
	)
	if i.serverAddress != "" {
		fields[keyServerAddress] = i.serverAddress
	}
	if i.serverPort > 0 {
		fields[keyServerPort] = i.serverPort
	}
	if i.dependencyName != "" {
		fields[keyApplicationDependencyName] = i.dependencyName
	}
	return fields
}

type rpcOutcome struct {
	// rpc contains immutable transport facts captured before the call starts.
	rpc rpcInfo
	// duration measures the full logical call and is used by logs and aggregate metrics.
	duration time.Duration
	// responseStatusCode is the native gRPC status resolved from the returned error.
	responseStatusCode codes.Code
	// callError is the exact error returned by the gRPC client call.
	callError error
	// responseStatusMessage is the native gRPC status description returned by the dependency.
	responseStatusMessage string
}

func newRPCOutcome(rpc rpcInfo, duration time.Duration, err error) rpcOutcome {
	return rpcOutcome{
		rpc:                   rpc,
		duration:              duration,
		responseStatusCode:    status.Code(err),
		callError:             err,
		responseStatusMessage: status.Convert(err).Message(),
	}
}

func (o rpcOutcome) responseStatusName() string {
	return grpcsemconv.StatusName(o.responseStatusCode)
}

func (o rpcOutcome) errorType() string {
	return grpcsemconv.ClientErrorType(o.responseStatusCode)
}

func streamType(info *googlegrpc.StreamDesc) string {
	switch {
	case info.ClientStreams && info.ServerStreams:
		return "bidi_stream"
	case info.ClientStreams:
		return "client_stream"
	default:
		return "server_stream"
	}
}
