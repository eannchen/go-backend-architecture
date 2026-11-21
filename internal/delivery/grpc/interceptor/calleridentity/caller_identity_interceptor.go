package calleridentity

import (
	"context"
	"crypto/x509"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/eannchen/go-backend-architecture/internal/delivery/grpc/grpccontext"
	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
)

// Interceptor publishes verified mTLS caller identities to downstream contexts.
type Interceptor struct {
	responder *grpcresponse.Responder
}

// New creates caller-identity interceptors using the URI SAN identity policy.
func New(responder *grpcresponse.Responder) *Interceptor {
	return &Interceptor{responder: responder}
}

// Unary enriches unary RPC contexts when the TLS handshake verified a client certificate.
func (i *Interceptor) Unary() googlegrpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *googlegrpc.UnaryServerInfo,
		handler googlegrpc.UnaryHandler,
	) (any, error) {
		requestCtx, err := i.enrich(ctx)
		if err != nil {
			return nil, err
		}
		return handler(requestCtx, req)
	}
}

// Stream enriches streaming RPC contexts when the TLS handshake verified a client certificate.
func (i *Interceptor) Stream() googlegrpc.StreamServerInterceptor {
	return func(
		srv any,
		stream googlegrpc.ServerStream,
		_ *googlegrpc.StreamServerInfo,
		handler googlegrpc.StreamHandler,
	) error {
		requestCtx, err := i.enrich(stream.Context())
		if err != nil {
			return err
		}
		return handler(srv, &contextServerStream{ServerStream: stream, ctx: requestCtx})
	}
}

func (i *Interceptor) enrich(ctx context.Context) (context.Context, error) {
	certificate := verifiedClientCertificate(ctx)
	if certificate == nil {
		return ctx, nil
	}
	identity, err := extractCallerIdentity(certificate)
	if err != nil {
		return ctx, i.responder.Error(err, codes.Unauthenticated, "client certificate has no usable identity")
	}
	return grpccontext.WithCallerIdentity(ctx, identity), nil
}

func verifiedClientCertificate(ctx context.Context) *x509.Certificate {
	peerInfo, ok := peer.FromContext(ctx)
	if !ok || peerInfo.AuthInfo == nil {
		return nil
	}

	var tlsInfo credentials.TLSInfo
	switch authInfo := peerInfo.AuthInfo.(type) {
	case credentials.TLSInfo:
		tlsInfo = authInfo
	case *credentials.TLSInfo:
		if authInfo == nil {
			return nil
		}
		tlsInfo = *authInfo
	default:
		return nil
	}
	// PeerCertificates also contains unverified certificates. VerifiedChains is
	// the handshake result that proves the configured client CA trusted the peer.
	if len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return nil
	}
	// Verification may produce several valid paths through intermediate CAs, but
	// every path starts with the same peer leaf certificate. The first [0] picks
	// one verified path; the second [0] picks that path's caller certificate.
	return tlsInfo.State.VerifiedChains[0][0]
}

type contextServerStream struct {
	googlegrpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }
