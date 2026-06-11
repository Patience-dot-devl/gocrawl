package crawler

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"HTTP://Example.com":           "http://example.com/",
		"https://example.com/":         "https://example.com/",
		"https://example.com/a/":       "https://example.com/a",
		"https://example.com/a#frag":   "https://example.com/a",
		"https://example.com:443/path": "https://example.com/path",
		"http://example.com:80/":       "http://example.com/",
		"https://example.com/a?b=1":    "https://example.com/a?b=1",
	}
	for in, want := range cases {
		if got := normalizeURL(in, false); got != want {
			t.Errorf("normalizeURL(%q, false) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeURLStripQuery(t *testing.T) {
	cases := map[string]string{
		"https://example.com/a?b=1":         "https://example.com/a",
		"https://example.com/a?b=1&c=2":     "https://example.com/a",
		"https://example.com/a":             "https://example.com/a",
		"https://example.com/?utm_source=x": "https://example.com/",
		"https://example.com/a?#frag":       "https://example.com/a",
	}
	for in, want := range cases {
		if got := normalizeURL(in, true); got != want {
			t.Errorf("normalizeURL(%q, true) = %q, want %q", in, got, want)
		}
	}
}

func TestSameSite(t *testing.T) {
	if !sameSite("example.com", "example.com", false) {
		t.Error("identical hosts should be same site")
	}
	if sameSite("example.com", "blog.example.com", false) {
		t.Error("subdomain should not match when allowSubdomains=false")
	}
	if !sameSite("example.com", "blog.example.com", true) {
		t.Error("subdomain should match when allowSubdomains=true")
	}
	if sameSite("example.com", "evil.com", true) {
		t.Error("different registrable domains should never match")
	}
}
