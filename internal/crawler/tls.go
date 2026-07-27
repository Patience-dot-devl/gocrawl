package crawler

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"time"
)

// TLSInfo is a snapshot of the TLS handshake that carried a page's final response: the
// negotiated protocol version and cipher suite, the ALPN protocol, and the certificate chain
// the server presented (leaf first). The engine captures it and knows nothing more about it;
// interpreting it is the security analyzer's opt-in audit mode's job.
//
// It is deliberately not serialized into reports: every page of a crawl shares one host's
// handshake, so repeating an identical chain per page would bloat the JSON for no benefit.
// Findings derived from it travel as analyze.Issue values instead.
type TLSInfo struct {
	Version     uint16
	CipherSuite uint16
	// ALPN is the application protocol negotiated during the handshake ("h2", "http/1.1"), or
	// empty when the server offered none.
	ALPN string
	// Chain is the certificate chain the server presented, leaf first. It is what the server
	// actually sent, not the verified chain, so a server that omits its intermediates shows up
	// here as a one-element chain even though the fetch succeeded.
	Chain []CertInfo
}

// VersionName renders the negotiated protocol version ("TLS 1.3").
func (t *TLSInfo) VersionName() string { return tls.VersionName(t.Version) }

// CipherName renders the negotiated cipher suite ("TLS_AES_128_GCM_SHA256").
func (t *TLSInfo) CipherName() string { return tls.CipherSuiteName(t.CipherSuite) }

// Leaf returns the server (end-entity) certificate, or nil when the chain is empty.
func (t *TLSInfo) Leaf() *CertInfo {
	if t == nil || len(t.Chain) == 0 {
		return nil
	}
	return &t.Chain[0]
}

// CertInfo is the subset of an X.509 certificate the security audit reads. The full
// certificate is not retained: the audit only needs identity, validity window, and the
// strength of the signature and key.
type CertInfo struct {
	Subject            string
	Issuer             string
	NotBefore          time.Time
	NotAfter           time.Time
	SignatureAlgorithm x509.SignatureAlgorithm
	// KeyType and KeyBits describe the public key ("RSA"/2048, "ECDSA"/256, "Ed25519"/256).
	// KeyBits is 0 for a key type we don't measure.
	KeyType string
	KeyBits int
	IsCA    bool
	// SelfSigned is true when issuer and subject are the same distinguished name — either a
	// root CA at the top of the chain, or an untrusted self-signed server certificate.
	SelfSigned bool
}

// newTLSInfo distills a completed handshake into the TLSInfo stored on a Page. It returns
// nil for a plaintext connection.
func newTLSInfo(cs *tls.ConnectionState) *TLSInfo {
	if cs == nil {
		return nil
	}
	info := &TLSInfo{
		Version:     cs.Version,
		CipherSuite: cs.CipherSuite,
		ALPN:        cs.NegotiatedProtocol,
		Chain:       make([]CertInfo, 0, len(cs.PeerCertificates)),
	}
	for _, c := range cs.PeerCertificates {
		info.Chain = append(info.Chain, newCertInfo(c))
	}
	return info
}

func newCertInfo(c *x509.Certificate) CertInfo {
	ci := CertInfo{
		Subject:            certName(c.Subject),
		Issuer:             certName(c.Issuer),
		NotBefore:          c.NotBefore,
		NotAfter:           c.NotAfter,
		SignatureAlgorithm: c.SignatureAlgorithm,
		IsCA:               c.IsCA,
		SelfSigned:         bytes.Equal(c.RawIssuer, c.RawSubject),
	}
	switch pub := c.PublicKey.(type) {
	case *rsa.PublicKey:
		ci.KeyType, ci.KeyBits = "RSA", pub.N.BitLen()
	case *ecdsa.PublicKey:
		ci.KeyType, ci.KeyBits = "ECDSA", pub.Curve.Params().BitSize
	case ed25519.PublicKey:
		ci.KeyType, ci.KeyBits = "Ed25519", 256
	}
	return ci
}

// certName renders a distinguished name compactly, preferring the common name and falling
// back to the organization (some CA certificates carry no CN) and then the full DN.
func certName(n pkix.Name) string {
	if n.CommonName != "" {
		return n.CommonName
	}
	if len(n.Organization) > 0 {
		return n.Organization[0]
	}
	return n.String()
}
