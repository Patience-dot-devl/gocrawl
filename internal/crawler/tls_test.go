package crawler

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestFetchCapturesTLSHandshake drives the real Fetch path over a real TLS connection and
// asserts the handshake lands on the Page. The fetcher's client transport is swapped for the
// test server's so the self-signed test certificate verifies; everything else — the request,
// the handshake, the capture — is the production code path.
func TestFetchCapturesTLSHandshake(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>hi</body></html>"))
	}))
	defer srv.Close()

	f := NewHTTPFetcher(DefaultOptions())
	f.client.Transport = srv.Client().Transport

	page, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if page.TLS == nil {
		t.Fatal("page.TLS is nil, want the captured handshake")
	}
	if page.TLS.Version < tls.VersionTLS12 {
		t.Errorf("Version = %s, want TLS 1.2 or better", page.TLS.VersionName())
	}
	if page.TLS.CipherName() == "" {
		t.Error("CipherName is empty, want the negotiated suite")
	}
	leaf := page.TLS.Leaf()
	if leaf == nil {
		t.Fatal("Leaf() = nil, want the server certificate")
	}
	if leaf.NotAfter.IsZero() {
		t.Error("leaf NotAfter is zero, want the certificate's expiry")
	}
	if leaf.KeyBits == 0 {
		t.Errorf("leaf KeyBits = 0, want a measured key size (KeyType %q)", leaf.KeyType)
	}
}

// TestFetchPlainHTTPHasNoTLS confirms an unencrypted fetch leaves TLS nil rather than
// fabricating an empty handshake, which the audit relies on to tell the two apart.
func TestFetchPlainHTTPHasNoTLS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()

	page, err := NewHTTPFetcher(DefaultOptions()).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if page.TLS != nil {
		t.Errorf("page.TLS = %+v over plain HTTP, want nil", page.TLS)
	}
}

func TestNewTLSInfoNilConnectionState(t *testing.T) {
	if got := newTLSInfo(nil); got != nil {
		t.Errorf("newTLSInfo(nil) = %+v, want nil", got)
	}
}

// TestNewCertInfoDistillsCertificate checks the fields the security audit thresholds on,
// across the key types a real chain mixes.
func TestNewCertInfoDistillsCertificate(t *testing.T) {
	notBefore := time.Now().Add(-24 * time.Hour)
	notAfter := time.Now().Add(90 * 24 * time.Hour)

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}

	// A self-signed CA, then a leaf it issues — the two shapes the audit distinguishes.
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gocrawl Test CA"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	ca := mustCert(t, caTmpl, caTmpl, &rsaKey.PublicKey, rsaKey)

	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "example.test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		DNSNames:     []string{"example.test"},
	}
	leaf := mustCert(t, leafTmpl, ca, &ecKey.PublicKey, rsaKey)

	gotCA := newCertInfo(ca)
	if gotCA.Subject != "gocrawl Test CA" {
		t.Errorf("CA Subject = %q, want %q", gotCA.Subject, "gocrawl Test CA")
	}
	if !gotCA.SelfSigned {
		t.Error("CA SelfSigned = false, want true (issuer equals subject)")
	}
	if !gotCA.IsCA {
		t.Error("CA IsCA = false, want true")
	}
	if gotCA.KeyType != "RSA" || gotCA.KeyBits != 2048 {
		t.Errorf("CA key = %s/%d, want RSA/2048", gotCA.KeyType, gotCA.KeyBits)
	}

	gotLeaf := newCertInfo(leaf)
	if gotLeaf.SelfSigned {
		t.Error("leaf SelfSigned = true, want false (issued by the CA)")
	}
	if gotLeaf.Issuer != "gocrawl Test CA" {
		t.Errorf("leaf Issuer = %q, want %q", gotLeaf.Issuer, "gocrawl Test CA")
	}
	if gotLeaf.KeyType != "ECDSA" || gotLeaf.KeyBits != 256 {
		t.Errorf("leaf key = %s/%d, want ECDSA/256", gotLeaf.KeyType, gotLeaf.KeyBits)
	}
	if !gotLeaf.NotAfter.Equal(notAfter.Truncate(time.Second).UTC()) {
		t.Errorf("leaf NotAfter = %v, want %v", gotLeaf.NotAfter, notAfter.Truncate(time.Second).UTC())
	}
}

// TestCertNameFallsBackToOrganization covers the CA certificates that carry no common name.
func TestCertNameFallsBackToOrganization(t *testing.T) {
	if got := certName(pkix.Name{Organization: []string{"Example Trust Services"}}); got != "Example Trust Services" {
		t.Errorf("certName(no CN) = %q, want the organization", got)
	}
}

// mustCert signs tmpl with parent and returns the parsed certificate.
func mustCert(t *testing.T, tmpl, parent *x509.Certificate, pub any, signer any) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}
