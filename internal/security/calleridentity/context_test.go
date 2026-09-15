package calleridentity

import (
	"context"
	"testing"
)

func TestIdentityRoundTripsThroughContext(t *testing.T) {
	want := Identity{Subject: "spiffe://example.internal/service/catalog", AuthenticationType: AuthenticationTypeMTLS}

	got, ok := FromContext(WithIdentity(context.Background(), want))
	if !ok || got != want {
		t.Fatalf("FromContext() = (%+v, %t), want (%+v, true)", got, ok, want)
	}
}

func TestFromContextReportsMissingIdentity(t *testing.T) {
	if got, ok := FromContext(context.Background()); ok {
		t.Fatalf("FromContext() = (%+v, true), want no identity", got)
	}
}
