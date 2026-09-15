package calleridentity

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	securityidentity "github.com/eannchen/go-backend-architecture/internal/security/calleridentity"
)

func TestUnaryPublishesVerifiedURIIdentity(t *testing.T) {
	certificate := &x509.Certificate{}
	ctx := verifiedTLSContext(certificate)
	interceptor := newTestInterceptor(t, successfulExtractor)

	_, err := interceptor.Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		identity, ok := securityidentity.FromContext(ctx)
		if !ok {
			t.Fatal("handler context has no caller identity")
		}
		if identity.Subject != "spiffe://example.internal/service/catalog" || identity.AuthenticationType != securityidentity.AuthenticationTypeMTLS {
			t.Fatalf("caller identity = %+v", identity)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

func TestUnaryDoesNotTrustPresentedUnverifiedCertificate(t *testing.T) {
	certificate := &x509.Certificate{}
	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: credentials.TLSInfo{
		State: tlsConnectionStateWithPeerCertificate(certificate),
	}})
	interceptor := newTestInterceptor(t, successfulExtractor)

	_, err := interceptor.Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		if identity, ok := securityidentity.FromContext(ctx); ok {
			t.Fatalf("unverified certificate established identity %+v", identity)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

func TestUnaryRejectsCertificateIdentityExtractionFailure(t *testing.T) {
	ctx := verifiedTLSContext(&x509.Certificate{})
	interceptor := newTestInterceptor(t, func(*x509.Certificate) (securityidentity.Identity, error) {
		return securityidentity.Identity{}, errors.New("ambiguous certificate identity")
	})
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
}

func TestStreamPublishesVerifiedURIIdentity(t *testing.T) {
	ctx := verifiedTLSContext(&x509.Certificate{})
	stream := testServerStream{ctx: ctx}
	interceptor := newTestInterceptor(t, successfulExtractor)

	err := interceptor.Stream()(nil, stream, &googlegrpc.StreamServerInfo{}, func(_ any, stream googlegrpc.ServerStream) error {
		identity, ok := securityidentity.FromContext(stream.Context())
		if !ok || identity.Subject != "spiffe://example.internal/service/catalog" {
			t.Fatalf("caller identity = (%+v, %t)", identity, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

func TestNewRejectsMissingCertificateExtractor(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Fatal("New() error = nil, want missing extractor error")
	}
}

func newTestInterceptor(
	t *testing.T,
	extract func(*x509.Certificate) (securityidentity.Identity, error),
) *Interceptor {
	t.Helper()
	interceptor, err := New(certificateExtractorFunc(extract), nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return interceptor
}

type certificateExtractorFunc func(*x509.Certificate) (securityidentity.Identity, error)

func (f certificateExtractorFunc) Extract(certificate *x509.Certificate) (securityidentity.Identity, error) {
	return f(certificate)
}

func successfulExtractor(*x509.Certificate) (securityidentity.Identity, error) {
	return securityidentity.Identity{
		Subject:            "spiffe://example.internal/service/catalog",
		AuthenticationType: securityidentity.AuthenticationTypeMTLS,
	}, nil
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
