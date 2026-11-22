package grpccontext

import (
	"context"
	"testing"
)

// TestIdentityRoundTripsThroughContext checks verified caller identity can be passed through context safely.
func TestIdentityRoundTripsThroughContext(t *testing.T) {
	want := CallerIdentity{Subject: "spiffe://example.internal/service/catalog", AuthenticationType: AuthenticationTypeMTLS}

	got, ok := CallerIdentityFromContext(WithCallerIdentity(context.Background(), want))
	if !ok || got != want {
		t.Fatalf("CallerIdentityFromContext() = (%+v, %t), want (%+v, true)", got, ok, want)
	}
}

// TestCallerIdentityFromContextReportsMissingIdentity checks absent identity is reported explicitly to downstream interceptors and services.
func TestCallerIdentityFromContextReportsMissingIdentity(t *testing.T) {
	if got, ok := CallerIdentityFromContext(context.Background()); ok {
		t.Fatalf("CallerIdentityFromContext() = (%+v, true), want no identity", got)
	}
}
