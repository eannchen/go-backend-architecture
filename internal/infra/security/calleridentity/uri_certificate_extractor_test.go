package calleridentity

import (
	"crypto/x509"
	"net/url"
	"testing"

	securityidentity "github.com/eannchen/go-backend-architecture/internal/security/calleridentity"
)

func TestURICertificateExtractorExtract(t *testing.T) {
	tests := []struct {
		name        string
		certificate *x509.Certificate
		wantSubject string
		wantErr     bool
	}{
		{name: "missing certificate", wantErr: true},
		{name: "missing URI SAN", certificate: &x509.Certificate{}, wantErr: true},
		{
			name: "ambiguous URI SANs",
			certificate: &x509.Certificate{URIs: []*url.URL{
				mustParseURL(t, "spiffe://example.internal/service/a"),
				mustParseURL(t, "spiffe://example.internal/service/b"),
			}},
			wantErr: true,
		},
		{
			name:        "one URI SAN",
			certificate: &x509.Certificate{URIs: []*url.URL{mustParseURL(t, "spiffe://example.internal/service/catalog")}},
			wantSubject: "spiffe://example.internal/service/catalog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, err := (URICertificateExtractor{}).Extract(tt.certificate)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Extract() error = %v, want error %t", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if identity.Subject != tt.wantSubject || identity.AuthenticationType != securityidentity.AuthenticationTypeMTLS {
				t.Fatalf("Extract() = %+v", identity)
			}
		})
	}
}

func mustParseURL(t *testing.T, rawURI string) *url.URL {
	t.Helper()
	uri, err := url.Parse(rawURI)
	if err != nil {
		t.Fatalf("parse URI: %v", err)
	}
	return uri
}
