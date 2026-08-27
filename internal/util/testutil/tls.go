package testutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// CertificateAuthority issues short-lived certificates for transport tests.
type CertificateAuthority struct {
	certificate *x509.Certificate
	privateKey  *ecdsa.PrivateKey
	pem         []byte
}

// NewCertificateAuthority creates a short-lived test root.
func NewCertificateAuthority(t testing.TB) *CertificateAuthority {
	t.Helper()
	privateKey := newPrivateKey(t)
	template := &x509.Certificate{
		SerialNumber:          newSerialNumber(t),
		Subject:               pkix.Name{CommonName: "test root CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der := createCertificate(t, template, template, &privateKey.PublicKey, privateKey)
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse test CA certificate: %v", err)
	}
	return &CertificateAuthority{
		certificate: certificate,
		privateKey:  privateKey,
		pem:         pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}
}

// IssueServerCertificate creates a server certificate for the supplied DNS names.
func (a *CertificateAuthority) IssueServerCertificate(t testing.TB, dnsNames ...string) tls.Certificate {
	t.Helper()
	return a.issue(t, "test server", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, dnsNames)
}

// IssueClientCertificate creates a client-authentication certificate.
func (a *CertificateAuthority) IssueClientCertificate(t testing.TB, commonName string) tls.Certificate {
	t.Helper()
	return a.issue(t, commonName, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil)
}

// CertPool returns a trust pool containing this authority.
func (a *CertificateAuthority) CertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(a.certificate)
	return pool
}

// WriteCAFile writes the authority certificate to a temporary PEM file.
func (a *CertificateAuthority) WriteCAFile(t testing.TB) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	writeTestFile(t, path, a.pem)
	return path
}

// WriteCertificateFiles writes a certificate chain and private key to temporary PEM files.
func WriteCertificateFiles(t testing.TB, name string, certificate tls.Certificate) (certificateFile, privateKeyFile string) {
	t.Helper()
	directory := t.TempDir()
	certificateFile = filepath.Join(directory, name+".pem")
	privateKeyFile = filepath.Join(directory, name+"-key.pem")

	var certificatePEM []byte
	for _, der := range certificate.Certificate {
		certificatePEM = append(certificatePEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	privateKey, ok := certificate.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("test certificate private key type = %T, want ECDSA", certificate.PrivateKey)
	}
	privateKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal test private key: %v", err)
	}

	writeTestFile(t, certificateFile, certificatePEM)
	writeTestFile(t, privateKeyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDER}))
	return certificateFile, privateKeyFile
}

func (a *CertificateAuthority) issue(t testing.TB, commonName string, usages []x509.ExtKeyUsage, dnsNames []string) tls.Certificate {
	t.Helper()
	privateKey := newPrivateKey(t)
	template := &x509.Certificate{
		SerialNumber: newSerialNumber(t),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  usages,
	}
	der := createCertificate(t, template, a.certificate, &privateKey.PublicKey, a.privateKey)
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse issued test certificate: %v", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der, a.certificate.Raw},
		PrivateKey:  privateKey,
		Leaf:        leaf,
	}
}

func newPrivateKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate test private key: %v", err)
	}
	return privateKey
}

func newSerialNumber(t testing.TB) *big.Int {
	t.Helper()
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate test certificate serial number: %v", err)
	}
	return serialNumber
}

func createCertificate(t testing.TB, template, parent *x509.Certificate, publicKey any, parentPrivateKey any) []byte {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, template, parent, publicKey, parentPrivateKey)
	if err != nil {
		t.Fatalf("create test certificate: %v", err)
	}
	return der
}

func writeTestFile(t testing.TB, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write test certificate file: %v", err)
	}
}
