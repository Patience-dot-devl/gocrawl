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

func TestResultResolveHref(t *testing.T) {
	from := &Page{RequestedURL: "https://example.com/dir/page", FinalURL: "https://example.com/dir/page"}
	target := &Page{RequestedURL: "https://example.com/dir/other", FinalURL: "https://example.com/dir/other", StatusCode: 200}
	result := &Result{Pages: []*Page{from, target}}
	result.Reindex()

	t.Run("relative href resolves against from's FinalURL", func(t *testing.T) {
		got, resolved, ok := result.ResolveHref(from, "other")
		if !ok || got != target {
			t.Fatalf("ResolveHref(other) ok=%v got=%v, want target", ok, got)
		}
		if want := "https://example.com/dir/other"; resolved != want {
			t.Errorf("resolved = %q, want %q", resolved, want)
		}
	})

	t.Run("no match found", func(t *testing.T) {
		if _, _, ok := result.ResolveHref(from, "/nope"); ok {
			t.Error("expected no match for an uncrawled path")
		}
	})

	t.Run("unusable href", func(t *testing.T) {
		if _, _, ok := result.ResolveHref(from, "#fragment-only"); ok {
			t.Error("expected a fragment-only href to be unusable")
		}
		if _, _, ok := result.ResolveHref(from, "mailto:a@example.com"); ok {
			t.Error("expected a mailto: href to be unusable")
		}
	})

	t.Run("nil from", func(t *testing.T) {
		if _, _, ok := result.ResolveHref(nil, "other"); ok {
			t.Error("expected ResolveHref with a nil page to report not found")
		}
	})

	t.Run("no index built", func(t *testing.T) {
		unindexed := &Result{Pages: []*Page{from, target}}
		if _, _, ok := unindexed.ResolveHref(from, "other"); ok {
			t.Error("expected ResolveHref to report not found without Reindex/a live crawl index")
		}
	})
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

func TestStripPort(t *testing.T) {
	cases := map[string]string{
		"example.com":       "example.com",
		"example.com:8080":  "example.com",
		"[::1]":             "[::1]",
		"[::1]:8080":        "[::1]",
		"[2001:db8::1]:443": "[2001:db8::1]",
		"::1":               "::1", // bare IPv6, no unambiguous port separator: left alone
	}
	for in, want := range cases {
		if got := stripPort(in); got != want {
			t.Errorf("stripPort(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSameSiteIPv6 guards against a real bug: stripPort used strings.LastIndex(host, ":"),
// which for a bracketed IPv6 literal with no port (e.g. "[::1]") lands inside the address
// itself and corrupts it, breaking the same-site comparison.
func TestSameSiteIPv6(t *testing.T) {
	if !sameSite("[::1]", "[::1]", false) {
		t.Error("identical bracketed IPv6 hosts (no port) should be same site")
	}
	if !sameSite("[::1]:8080", "[::1]", false) {
		t.Error("bracketed IPv6 hosts differing only by port should be same site")
	}
}
