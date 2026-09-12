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
	dependency string
	fullMethod string
	service    string
	method     string
	rpcType    string
}

func newRPCInfo(dependency, fullMethod, rpcType string) rpcInfo {
	service, method := splitFullMethod(fullMethod)
	return rpcInfo{dependency: dependency, fullMethod: fullMethod, service: service, method: method, rpcType: rpcType}
}

func (i rpcInfo) fields() appobservability.Fields {
	fields := appobservability.FromPairs(
		keyRPCSystem, "grpc",
		keyRPCService, i.service,
		keyRPCMethod, i.method,
		keyRPCType, i.rpcType,
	)
	if i.dependency != "" {
		fields[keyDependencyName] = i.dependency
	}
	return fields
}

type rpcOutcome struct {
	rpc      rpcInfo
	duration time.Duration
	status   codes.Code
	err      error
	message  string
}

func newRPCOutcome(rpc rpcInfo, duration time.Duration, err error) rpcOutcome {
	return rpcOutcome{
		rpc:      rpc,
		duration: duration,
		status:   status.Code(err),
		err:      err,
		message:  status.Convert(err).Message(),
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
