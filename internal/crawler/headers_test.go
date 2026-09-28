package crawler

import (
	"strings"
	"testing"
)

// A Shopify crawler access key is three headers whose values carry commas, quotes, colons and
// semicolons; all of that must survive parsing intact.
func TestParseHeaderLinesKeepsSignatureValuesIntact(t *testing.T) {
	sigInput := `sig1=("@authority" "signature-agent");created=1759000000;expires=1766000000;keyid="abc,def";alg="ed25519";tag="web-bot-auth"`
	h, err := ParseHeaderLines([]string{
		"# UK store, expires 2026-12-17",
		"",
		"signature: sig1=:bWFkZS11cA==:",
		"Signature-Input: " + sigInput,
		"Signature-Agent:   https://shopify.com  ",
	})
	if err != nil {
		t.Fatalf("ParseHeaderLines: %v", err)
	}
	if got := h.Get("Signature"); got != "sig1=:bWFkZS11cA==:" {
		t.Errorf("Signature = %q", got)
	}
	if got := h.Get("Signature-Input"); got != sigInput {
		t.Errorf("Signature-Input = %q, want %q", got, sigInput)
	}
	if got := h.Get("Signature-Agent"); got != "https://shopify.com" {
		t.Errorf("Signature-Agent = %q, want trimmed https://shopify.com", got)
	}
	if len(h) != 3 {
		t.Errorf("got %d headers, want 3 (comment and blank line skipped): %v", len(h), h)
	}
}

func TestParseHeaderLinesLaterValueWins(t *testing.T) {
	h, err := ParseHeaderLines([]string{"X-Token: old", "x-token: new"})
	if err != nil {
		t.Fatalf("ParseHeaderLines: %v", err)
	}
	if got := h.Values("X-Token"); len(got) != 1 || got[0] != "new" {
		t.Errorf("X-Token = %v, want [new]", got)
	}
}

func TestParseHeaderLinesRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"missing colon":    "X-Token abc",
		"empty name":       ": abc",
		"space in name":    "X Token: abc",
		"header injection": "X-Token: abc\r\nX-Evil: 1",
		"newline in value": "X-Token: abc\nX-Evil: 1",
		"host":             "Host: other.example",
		"content-length":   "Content-Length: 0",
		"accept-encoding":  "Accept-Encoding: br",
		"cookie":           "Cookie: a=b",
		"user-agent":       "user-agent: Googlebot",
		"hop-by-hop":       "Connection: close",
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if h, err := ParseHeaderLines([]string{line}); err == nil {
				t.Errorf("ParseHeaderLines(%q) = %v, want an error", line, h)
			}
		})
	}
}

// Rejections name the dedicated option to use instead, since that's the fix.
func TestParseHeaderLinesPointsAtDedicatedOption(t *testing.T) {
	_, err := ParseHeaderLines([]string{"Cookie: a=b"})
	if err == nil || !strings.Contains(err.Error(), "--cookie") {
		t.Errorf("err = %v, want it to point at --cookie", err)
	}
}
