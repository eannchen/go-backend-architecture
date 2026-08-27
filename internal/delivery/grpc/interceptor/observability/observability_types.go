package observability

import (
	"strings"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

type rpcInfo struct {
	fullMethod string
	service    string
	method     string
	rpcType    string
}

func newRPCInfo(fullMethod, rpcType string) rpcInfo {
	service, method := splitFullMethod(fullMethod)
	return rpcInfo{
		fullMethod: fullMethod,
		service:    service,
		method:     method,
		rpcType:    rpcType,
	}
}

func (i rpcInfo) fields() appobservability.Fields {
	return appobservability.FromPairs(
		keyRPCSystem, "grpc",
		keyRPCService, i.service,
		keyRPCMethod, i.method,
		keyRPCType, i.rpcType,
	)
}

type rpcOutcome struct {
	rpc       rpcInfo
	duration  time.Duration
	code      codes.Code
	err       error
	errorInfo rpcError
}

func newRPCOutcome(rpc rpcInfo, duration time.Duration, err error) rpcOutcome {
	code := status.Code(err)
	return rpcOutcome{
		rpc:       rpc,
		duration:  duration,
		code:      code,
		err:       err,
		errorInfo: inspectRPCError(err, code),
	}
}

func splitFullMethod(fullMethod string) (string, string) {
	trimmed := strings.TrimPrefix(fullMethod, "/")
	separator := strings.LastIndexByte(trimmed, '/')
	if separator < 0 {
		return "unknown", trimmed
	}
	return trimmed[:separator], trimmed[separator+1:]
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
