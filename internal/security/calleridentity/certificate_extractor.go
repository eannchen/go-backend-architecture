package calleridentity

import "crypto/x509"

// CertificateExtractor maps a verified client certificate to a canonical caller identity.
type CertificateExtractor interface {
	Extract(*x509.Certificate) (Identity, error)
}
