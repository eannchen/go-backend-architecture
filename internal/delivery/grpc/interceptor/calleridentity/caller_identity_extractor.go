package calleridentity

import (
	"crypto/x509"
	"fmt"

	"github.com/eannchen/go-backend-architecture/internal/delivery/grpc/grpccontext"
)

// extractCallerIdentity receives a non-nil leaf certificate verified by the interceptor.
// Exactly one nonempty URI SAN must identify the caller.
func extractCallerIdentity(certificate *x509.Certificate) (grpccontext.CallerIdentity, error) {
	// A URI SAN is an explicit machine identity field and supports conventions
	// such as SPIFFE IDs. Requiring exactly one avoids choosing silently between
	// multiple identities that could carry different authorization privileges.
	if len(certificate.URIs) != 1 || certificate.URIs[0] == nil || certificate.URIs[0].String() == "" {
		return grpccontext.CallerIdentity{}, fmt.Errorf("client certificate must contain exactly one URI SAN")
	}
	return grpccontext.CallerIdentity{
		Subject:            certificate.URIs[0].String(),
		AuthenticationType: grpccontext.AuthenticationTypeMTLS,
	}, nil
}
