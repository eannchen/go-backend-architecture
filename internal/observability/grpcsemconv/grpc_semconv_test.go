package grpcsemconv

import (
	"testing"

	"google.golang.org/grpc/codes"
)

// TestNormalizeMethodRemovesOnlyWirePathPrefix checks only the gRPC wire prefix is removed from method names.
func TestNormalizeMethodRemovesOnlyWirePathPrefix(t *testing.T) {
	if got := NormalizeMethod("/diagnostics.v1.DiagnosticsService/GetHealth"); got != "diagnostics.v1.DiagnosticsService/GetHealth" {
		t.Fatalf("NormalizeMethod() = %q", got)
	}
}

// TestStatusNameUsesCanonicalProtobufName checks statuses use canonical protobuf names for stable telemetry.
func TestStatusNameUsesCanonicalProtobufName(t *testing.T) {
	if got := StatusName(codes.DeadlineExceeded); got != "DEADLINE_EXCEEDED" {
		t.Fatalf("StatusName() = %q, want DEADLINE_EXCEEDED", got)
	}
	if got := StatusName(codes.Code(99)); got != "99" {
		t.Fatalf("StatusName() for unknown code = %q, want 99", got)
	}
}

// TestErrorTypeUsesDifferentClientAndServerRules checks client and server failures produce the appropriate bounded error type.
func TestErrorTypeUsesDifferentClientAndServerRules(t *testing.T) {
	if got := ClientErrorType(codes.InvalidArgument); got != "INVALID_ARGUMENT" {
		t.Fatalf("ClientErrorType() = %q, want INVALID_ARGUMENT", got)
	}
	if got := ServerErrorType(codes.InvalidArgument); got != "" {
		t.Fatalf("ServerErrorType() = %q, want empty", got)
	}
	if got := ServerErrorType(codes.Internal); got != "INTERNAL" {
		t.Fatalf("ServerErrorType() = %q, want INTERNAL", got)
	}
}
