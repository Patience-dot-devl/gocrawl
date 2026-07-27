package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Renewal windows for the leaf certificate. Thirty days is when most operators want a
// reminder; inside fourteen an unattended renewal (ACME typically renews at 30 days) has
// already failed at least once, so the finding escalates to an error.
const (
	certRenewalWarn  = 30 * 24 * time.Hour
	certRenewalError = 14 * 24 * time.Hour
)

// Minimum key sizes. 2048-bit RSA and 256-bit elliptic curves are the CA/Browser Forum
// baseline; anything below is no longer issuable and is on borrowed time in browsers.
const (
	minRSABits   = 2048
	minCurveBits = 256
)

// weakSignatures are the certificate signature algorithms with practical collision attacks.
// Browsers reject them outright for publicly trusted certificates.
var weakSignatures = map[x509.SignatureAlgorithm]bool{
	x509.MD2WithRSA:    true,
	x509.MD5WithRSA:    true,
	x509.SHA1WithRSA:   true,
	x509.DSAWithSHA1:   true,
	x509.ECDSAWithSHA1: true,
}

// insecureCiphers is the set of cipher suites Go itself classifies as insecure (RC4, 3DES,
// and the CBC suites vulnerable to Lucky13 and friends). Deriving it from the standard
// library rather than hard-coding IDs keeps the check current with Go's own assessment.
var insecureCiphers = func() map[uint16]bool {
	m := make(map[uint16]bool)
	for _, cs := range tls.InsecureCipherSuites() {
		m[cs.ID] = true
	}
	return m
}()

// auditTLS inspects the handshake and certificate chain behind this host's responses.
//
// It reads only what the crawl already negotiated, which bounds what it can find: Go verifies
// certificates before returning a response, so a chain that is expired, self-signed, or issued
// for the wrong hostname normally surfaces as a fetch error (reported by the `redirects`
// analyzer as http-fetch-error) and never reaches this code. Those checks are kept anyway —
// they do fire when a crawl runs through a TLS-terminating proxy that supplies its own trust
// anchor, which is exactly the setup where a bad origin certificate would otherwise go unseen.
func (h host) auditTLS() []analyze.Issue {
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, h.issue(sev, code, msg, data))
	}

	secure, plaintext := h.schemes()
	if len(plaintext) > 0 {
		add(analyze.Error, "security-no-https",
			fmt.Sprintf("%d page(s) are served over plain HTTP without redirecting to HTTPS", len(plaintext)),
			map[string]any{"count": len(plaintext), "example": plaintext[0].FinalURL})
	}
	if len(secure) == 0 {
		return issues
	}

	info := h.tlsInfo()
	if info == nil {
		// HTTPS, but no handshake was captured — headless render mode, where responses come
		// from the browser rather than Go's TLS stack. Silent by design; runner.Run notes it
		// once for the whole report instead of once per host.
		return issues
	}

	switch {
	case info.Version < tls.VersionTLS12:
		add(analyze.Error, "security-tls-obsolete-version",
			fmt.Sprintf("Connection negotiated %s, an obsolete protocol version", info.VersionName()),
			map[string]any{"version": info.VersionName()})
	case info.Version == tls.VersionTLS12:
		add(analyze.Info, "security-tls-legacy-version",
			"Connection negotiated TLS 1.2; TLS 1.3 is not offered or not preferred",
			map[string]any{"version": info.VersionName()})
	}

	if insecureCiphers[info.CipherSuite] {
		add(analyze.Error, "security-tls-weak-cipher",
			fmt.Sprintf("Connection negotiated the insecure cipher suite %s", info.CipherName()),
			map[string]any{"cipher_suite": info.CipherName(), "version": info.VersionName()})
	}

	issues = append(issues, h.auditChain(info)...)
	return issues
}

// auditChain checks the certificate chain the server presented: the leaf's validity window
// and provenance, and the signature and key strength of every certificate the server sent.
func (h host) auditChain(info *crawler.TLSInfo) []analyze.Issue {
	leaf := info.Leaf()
	if leaf == nil {
		return nil
	}
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, h.issue(sev, code, msg, data))
	}

	now := time.Now()
	certData := map[string]any{
		"subject":    leaf.Subject,
		"issuer":     leaf.Issuer,
		"expires_at": leaf.NotAfter.UTC().Format(time.RFC3339),
	}
	healthy := true

	switch remaining := leaf.NotAfter.Sub(now); {
	case remaining <= 0:
		healthy = false
		add(analyze.Error, "security-tls-cert-expired",
			fmt.Sprintf("Certificate expired %s ago", humanDays(-remaining)), certData)
	case remaining < certRenewalError:
		healthy = false
		add(analyze.Error, "security-tls-cert-expiring-soon",
			fmt.Sprintf("Certificate expires in %s", humanDays(remaining)),
			withDays(certData, remaining))
	case remaining < certRenewalWarn:
		healthy = false
		add(analyze.Warning, "security-tls-cert-expiring-soon",
			fmt.Sprintf("Certificate expires in %s", humanDays(remaining)),
			withDays(certData, remaining))
	}

	if leaf.NotBefore.After(now) {
		healthy = false
		add(analyze.Error, "security-tls-cert-not-yet-valid",
			fmt.Sprintf("Certificate is not valid until %s", leaf.NotBefore.UTC().Format(time.RFC3339)), certData)
	}

	if leaf.SelfSigned {
		healthy = false
		add(analyze.Error, "security-tls-cert-self-signed",
			"Server presented a self-signed certificate", certData)
	} else if len(info.Chain) == 1 {
		healthy = false
		add(analyze.Warning, "security-tls-incomplete-chain",
			"Server sent only its leaf certificate, without the issuing intermediate(s)",
			map[string]any{"subject": leaf.Subject, "issuer": leaf.Issuer})
	}

	// Signature and key strength apply to every certificate the server sent, not just the
	// leaf — a weak intermediate undermines the chain just as effectively. Self-signed roots
	// are exempt: a root is trusted by identity, not by its own signature, and long-lived
	// SHA-1 roots are still common in trust stores.
	for _, c := range info.Chain {
		isRoot := c.IsCA && c.SelfSigned
		if weakSignatures[c.SignatureAlgorithm] && !isRoot {
			healthy = false
			add(analyze.Error, "security-tls-weak-signature",
				fmt.Sprintf("Certificate %q is signed with %s, a broken algorithm", c.Subject, c.SignatureAlgorithm),
				map[string]any{"subject": c.Subject, "signature_algorithm": c.SignatureAlgorithm.String()})
		}
		if bits, weak := weakKey(c); weak {
			healthy = false
			add(analyze.Error, "security-tls-weak-key",
				fmt.Sprintf("Certificate %q uses a %d-bit %s key, below the %s minimum", c.Subject, bits, c.KeyType, keyFloor(c.KeyType)),
				map[string]any{"subject": c.Subject, "key_type": c.KeyType, "key_bits": bits})
		}
	}

	if healthy {
		add(analyze.Info, "security-tls-ok",
			fmt.Sprintf("%s with %s; certificate valid for another %s", info.VersionName(), info.CipherName(), humanDays(leaf.NotAfter.Sub(now))),
			withDays(map[string]any{
				"version":      info.VersionName(),
				"cipher_suite": info.CipherName(),
				"alpn":         info.ALPN,
				"issuer":       leaf.Issuer,
				"expires_at":   leaf.NotAfter.UTC().Format(time.RFC3339),
			}, leaf.NotAfter.Sub(now)))
	}
	return issues
}

// tlsInfo returns the first captured handshake for this host. Every page on a host shares one
// server configuration, so the first one that recorded a handshake speaks for all of them.
func (h host) tlsInfo() *crawler.TLSInfo {
	for _, p := range h.pages {
		if p.TLS != nil && len(p.TLS.Chain) > 0 {
			return p.TLS
		}
	}
	return nil
}

// weakKey reports whether a certificate's public key is below the CA/Browser Forum minimum
// for its type. Key types we don't measure (KeyBits 0) are never flagged.
func weakKey(c crawler.CertInfo) (bits int, weak bool) {
	switch c.KeyType {
	case "RSA":
		return c.KeyBits, c.KeyBits < minRSABits
	case "ECDSA":
		return c.KeyBits, c.KeyBits < minCurveBits
	}
	return c.KeyBits, false
}

// keyFloor names the minimum size for a key type, for the finding's message.
func keyFloor(keyType string) string {
	if keyType == "RSA" {
		return "2048-bit"
	}
	return "256-bit"
}

// humanDays renders a duration as whole days, falling back to hours under a day so a
// certificate hours from expiry doesn't read as "0 days".
func humanDays(d time.Duration) string {
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(math.Max(0, d.Hours())))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// withDays copies data with a days_remaining entry added, so a consumer can threshold on the
// number without re-parsing the message or the timestamp.
func withDays(data map[string]any, remaining time.Duration) map[string]any {
	out := make(map[string]any, len(data)+1)
	for k, v := range data {
		out[k] = v
	}
	out["days_remaining"] = int(remaining.Hours() / 24)
	return out
}
