package calleridentity

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"testing"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/delivery/grpc/grpccontext"
	"github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
)

// TestUnaryPublishesVerifiedURIIdentity checks a verified certificate URI becomes caller identity for unary handlers.
func TestUnaryPublishesVerifiedURIIdentity(t *testing.T) {
	certificate := validClientCertificate(t)
	ctx := verifiedTLSContext(certificate)
	interceptor := New(response.NewResponder())

	_, err := interceptor.Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		identity, ok := grpccontext.CallerIdentityFromContext(ctx)
		if !ok {
			t.Fatal("handler context has no caller identity")
		}
		if identity.Subject != "spiffe://example.internal/service/catalog" || identity.AuthenticationType != grpccontext.AuthenticationTypeMTLS {
			t.Fatalf("caller identity = %+v", identity)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

// TestUnaryDoesNotTrustPresentedUnverifiedCertificate checks a presented certificate without verification cannot establish caller identity.
func TestUnaryDoesNotTrustPresentedUnverifiedCertificate(t *testing.T) {
	certificate := validClientCertificate(t)
	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: credentials.TLSInfo{
		State: tlsConnectionStateWithPeerCertificate(certificate),
	}})
	interceptor := New(response.NewResponder())

	_, err := interceptor.Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		if identity, ok := grpccontext.CallerIdentityFromContext(ctx); ok {
			t.Fatalf("unverified certificate established identity %+v", identity)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

// TestUnaryRejectsInvalidCertificateIdentity checks invalid URI identities reject
// calls before the application handler can trust an authenticated caller.
func TestUnaryRejectsInvalidCertificateIdentity(t *testing.T) {
	tests := []struct {
		name        string
		certificate *x509.Certificate
	}{
		{name: "missing URI", certificate: &x509.Certificate{}},
		{name: "ambiguous URI", certificate: &x509.Certificate{URIs: []*url.URL{
			mustParseURL(t, "spiffe://example.internal/service/a"),
			mustParseURL(t, "spiffe://example.internal/service/b"),
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := verifiedTLSContext(tt.certificate)
			interceptor := New(response.NewResponder())
			handlerCalled := false
			_, err := interceptor.Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
				handlerCalled = true
				return nil, nil
			})
			if got := status.Code(err); got != codes.Unauthenticated {
				t.Fatalf("status = %s, want Unauthenticated", got)
			}
			if handlerCalled {
				t.Fatal("handler ran without a usable caller identity")
			}
		})
	}

}

// TestStreamPublishesVerifiedURIIdentity checks stream handlers receive the same verified identity as unary handlers.
func TestStreamPublishesVerifiedURIIdentity(t *testing.T) {
	ctx := verifiedTLSContext(validClientCertificate(t))
	stream := testServerStream{ctx: ctx}
	interceptor := New(response.NewResponder())

	err := interceptor.Stream()(nil, stream, &googlegrpc.StreamServerInfo{}, func(_ any, stream googlegrpc.ServerStream) error {
		identity, ok := grpccontext.CallerIdentityFromContext(stream.Context())
		if !ok || identity.Subject != "spiffe://example.internal/service/catalog" {
			t.Fatalf("caller identity = (%+v, %t)", identity, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

// TestStreamRejectsInvalidCertificateIdentity checks stream handlers cannot run
// when a verified certificate has no usable caller identity.
func TestStreamRejectsInvalidCertificateIdentity(t *testing.T) {
	stream := testServerStream{ctx: verifiedTLSContext(&x509.Certificate{})}
	interceptor := New(response.NewResponder())
	handlerCalled := false
	err := interceptor.Stream()(nil, stream, &googlegrpc.StreamServerInfo{}, func(any, googlegrpc.ServerStream) error {
		handlerCalled = true
		return nil
	})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("status = %s, want Unauthenticated", got)
	}
	if handlerCalled {
		t.Fatal("stream handler ran without a usable caller identity")
	}
}

func validClientCertificate(t *testing.T) *x509.Certificate {
	t.Helper()
	return &x509.Certificate{URIs: []*url.URL{mustParseURL(t, "spiffe://example.internal/service/catalog")}}
}

func verifiedTLSContext(certificate *x509.Certificate) context.Context {
	return peer.NewContext(context.Background(), &peer.Peer{AuthInfo: credentials.TLSInfo{
		State: tlsConnectionStateWithVerifiedChain(certificate),
	}})
}

func tlsConnectionStateWithVerifiedChain(certificate *x509.Certificate) tls.ConnectionState {
	return tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{certificate}}}
}

func tlsConnectionStateWithPeerCertificate(certificate *x509.Certificate) tls.ConnectionState {
	return tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}}
}

type testServerStream struct {
	googlegrpc.ServerStream
	ctx context.Context
}

func (s testServerStream) Context() context.Context { return s.ctx }
