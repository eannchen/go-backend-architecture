package grpcsemconv

import (
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
)

// NormalizeMethod converts gRPC's wire path into OTel's fully-qualified logical method.
func NormalizeMethod(fullMethod string) string {
	return strings.TrimPrefix(fullMethod, "/")
}

// StatusName returns the canonical protobuf name required by rpc.response.status_code.
func StatusName(code codes.Code) string {
	index := int(code)
	if index >= 0 && index < len(statusNames) {
		return statusNames[index]
	}
	return strconv.Itoa(index)
}

// ClientErrorType follows the OTel gRPC client rule that every non-OK status is a failed call.
func ClientErrorType(code codes.Code) string {
	if code == codes.OK {
		return ""
	}
	return StatusName(code)
}

// ServerErrorType follows the OTel gRPC server rule that only server-failure statuses fail the operation.
func ServerErrorType(code codes.Code) string {
	switch code {
	case codes.Unknown, codes.DeadlineExceeded, codes.Unimplemented, codes.Internal, codes.Unavailable, codes.DataLoss:
		return StatusName(code)
	default:
		return ""
	}
}

// statusNames follows the contiguous canonical gRPC code values from OK (0) through UNAUTHENTICATED (16).
var statusNames = [...]string{
	"OK", "CANCELLED", "UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED",
	"NOT_FOUND", "ALREADY_EXISTS", "PERMISSION_DENIED", "RESOURCE_EXHAUSTED",
	"FAILED_PRECONDITION", "ABORTED", "OUT_OF_RANGE", "UNIMPLEMENTED",
	"INTERNAL", "UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED",
}
