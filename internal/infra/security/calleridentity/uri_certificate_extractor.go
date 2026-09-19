package calleridentity

import (
	"crypto/x509"
	"fmt"

	securityidentity "github.com/eannchen/go-backend-architecture/internal/security/calleridentity"
)

// URICertificateExtractor maps one explicit URI SAN to an authenticated machine identity.
type URICertificateExtractor struct{}

// Extract implements the certificate identity contract using one URI SAN.
func (URICertificateExtractor) Extract(certificate *x509.Certificate) (securityidentity.Identity, error) {
	if certificate == nil {
		return securityidentity.Identity{}, fmt.Errorf("client certificate is missing")
	}
	// A URI SAN is an explicit machine identity field and supports conventions
	// such as SPIFFE IDs. Requiring exactly one avoids choosing silently between
	// multiple identities that could carry different authorization privileges.
	if len(certificate.URIs) != 1 || certificate.URIs[0] == nil || certificate.URIs[0].String() == "" {
		return securityidentity.Identity{}, fmt.Errorf("client certificate must contain exactly one URI SAN")
	}
	return securityidentity.Identity{
		Subject:            certificate.URIs[0].String(),
		AuthenticationType: securityidentity.AuthenticationTypeMTLS,
	}, nil
}
