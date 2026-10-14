package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestToOptionsLeavesHeadersNilByDefault(t *testing.T) {
	o, err := Default().ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if o.Headers != nil {
		t.Errorf("Headers = %v, want nil", o.Headers)
	}
}

// Profile, file, and inline headers merge in that order, a later source overriding an earlier
// one by name — so an inline --header can patch a single value from a stored file.
func TestToOptionsMergesHeaderSources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeFile(t, filepath.Join(home, ".gocrawl", "headers", "uk.headers"),
		"# UK store\r\nSignature: from-profile\r\nSignature-Agent: https://shopify.com\r\nX-Source: profile\r\n")
	file := filepath.Join(t.TempDir(), "extra.headers")
	writeFile(t, file, "X-Source: file\nX-File-Only: yes\n")

	c := Default()
	c.Crawl.HeaderProfile = "uk"
	c.Crawl.HeaderFile = file
	c.Crawl.Headers = []string{"Signature: inline"}
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	want := map[string]string{
		"Signature":       "inline",
		"Signature-Agent": "https://shopify.com",
		"X-Source":        "file",
		"X-File-Only":     "yes",
	}
	for name, v := range want {
		if got := o.Headers.Get(name); got != v {
			t.Errorf("%s = %q, want %q", name, got, v)
		}
	}
	if len(o.Headers) != len(want) {
		t.Errorf("got %d headers, want %d: %v", len(o.Headers), len(want), o.Headers)
	}
}

// A profile is a name, not a path: it must never resolve outside the profile directory, since
// the MCP and web APIs accept it from callers.
func TestHeaderProfilePathRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../secrets", "a/b", "/etc/passwd", ".hidden", "..", "", "uk store"} {
		if p, err := HeaderProfilePath(name); err == nil {
			t.Errorf("HeaderProfilePath(%q) = %q, want an error", name, p)
		}
	}
	p, err := HeaderProfilePath("uk-2026.q4")
	if err != nil {
		t.Fatalf("HeaderProfilePath: %v", err)
	}
	if filepath.Dir(p) != HeaderProfileDir() || filepath.Base(p) != "uk-2026.q4.headers" {
		t.Errorf("HeaderProfilePath = %q, want uk-2026.q4.headers inside %q", p, HeaderProfileDir())
	}
}

func TestToOptionsReportsMissingHeaderProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := Default()
	c.Crawl.HeaderProfile = "nope"
	if _, err := c.ToOptions(); err == nil || err.Error() != `header profile "nope" not found` {
		t.Errorf("err = %v, want a plain header profile not-found error", err)
	}
}

func TestToOptionsRejectsInvalidHeader(t *testing.T) {
	c := Default()
	c.Crawl.Headers = []string{"Host: evil.example"}
	if _, err := c.ToOptions(); err == nil {
		t.Error("expected an error for a reserved header")
	}
}

func TestToOptionsRejectsAuthorizationHeaderWithBasicAuth(t *testing.T) {
	c := Default()
	c.Crawl.BasicAuth = "alice:s3cret"
	c.Crawl.Headers = []string{"Authorization: Bearer abc"}
	if _, err := c.ToOptions(); err == nil {
		t.Error("expected an error combining an Authorization header with basic_auth")
	}

	c.Crawl.BasicAuth = ""
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if got := o.Headers.Get("Authorization"); got != "Bearer abc" {
		t.Errorf("Authorization = %q, want Bearer abc (allowed without basic_auth)", got)
	}
}

// Header values routinely contain commas, which viper's list decoding would split, so the env
// route for secrets is a header file — check that one is picked up.
func TestLoadPicksUpHeaderFileAndProfileFromEnv(t *testing.T) {
	t.Setenv("GOCRAWL_CRAWL_HEADER_FILE", "/tmp/uk.headers")
	t.Setenv("GOCRAWL_CRAWL_HEADER_PROFILE", "uk")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Crawl.HeaderFile != "/tmp/uk.headers" {
		t.Errorf("Crawl.HeaderFile = %q, want /tmp/uk.headers", cfg.Crawl.HeaderFile)
	}
	if cfg.Crawl.HeaderProfile != "uk" {
		t.Errorf("Crawl.HeaderProfile = %q, want uk", cfg.Crawl.HeaderProfile)
	}
}

func TestLoadReadsHeadersListFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gocrawl.yaml")
	writeFile(t, path, "crawl:\n  headers:\n    - 'Signature-Input: sig1=(\"@authority\");keyid=\"a,b\"'\n    - 'X-Two: 2'\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{`Signature-Input: sig1=("@authority");keyid="a,b"`, "X-Two: 2"}
	if !equalSlices(cfg.Crawl.Headers, want) {
		t.Errorf("Crawl.Headers = %q, want %q", cfg.Crawl.Headers, want)
	}
}

// A bearer token becomes an Authorization header on the custom-headers path, so it inherits
// that path's host restriction (crawler.HTTPFetcher.authHostAllowed). A pasted "Bearer "
// prefix is accepted rather than doubled.
func TestToOptionsBearerTokenSetsAuthorization(t *testing.T) {
	for _, token := range []string{"tok-123", "  tok-123  ", "Bearer tok-123", "bearer tok-123"} {
		c := Default()
		c.Crawl.BearerToken = token
		o, err := c.ToOptions()
		if err != nil {
			t.Fatalf("ToOptions(%q): %v", token, err)
		}
		if got := o.Headers.Get("Authorization"); got != "Bearer tok-123" {
			t.Errorf("BearerToken %q: Authorization = %q, want %q", token, got, "Bearer tok-123")
		}
	}
}

func TestToOptionsBearerTokenKeepsOtherHeaders(t *testing.T) {
	c := Default()
	c.Crawl.BearerToken = "tok"
	c.Crawl.Headers = []string{"X-Store: uk"}
	o, err := c.ToOptions()
	if err != nil {
		t.Fatalf("ToOptions: %v", err)
	}
	if o.Headers.Get("X-Store") != "uk" || o.Headers.Get("Authorization") != "Bearer tok" {
		t.Errorf("Headers = %v, want X-Store and the bearer Authorization", o.Headers)
	}
}

// Basic Auth and a custom Authorization header set the same header as the bearer token, so
// the combination is an error. The error must never echo the token: it reaches the CLI, MCP
// tool errors, and web API responses.
func TestToOptionsBearerTokenConflicts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeFile(t, filepath.Join(home, ".gocrawl", "headers", "auth.headers"), "Authorization: Basic eDp5\n")
	const token = "very-secret-token"
	cases := []struct {
		name string
		set  func(*Config)
	}{
		{"basic_auth", func(c *Config) { c.Crawl.BasicAuth = "alice:pw" }},
		{"inline Authorization header", func(c *Config) { c.Crawl.Headers = []string{"Authorization: Token abc"} }},
		{"Authorization from a header profile", func(c *Config) { c.Crawl.HeaderProfile = "auth" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			c.Crawl.BearerToken = token
			tc.set(&c)
			_, err := c.ToOptions()
			if err == nil {
				t.Fatal("ToOptions succeeded, want a conflict error")
			}
			if !strings.Contains(err.Error(), "bearer_token") {
				t.Errorf("error %q doesn't name bearer_token", err)
			}
			if strings.Contains(err.Error(), token) {
				t.Errorf("error %q leaks the token", err)
			}
		})
	}
}

func TestToOptionsBearerTokenRejectsHeaderInjection(t *testing.T) {
	c := Default()
	c.Crawl.BearerToken = "zq9-secret\r\nX-Evil: 1"
	_, err := c.ToOptions()
	if err == nil {
		t.Fatal("ToOptions accepted a token containing CR/LF")
	}
	if strings.Contains(err.Error(), "X-Evil") || strings.Contains(err.Error(), "zq9-secret") {
		t.Errorf("error %q echoes the token", err)
	}
}

// A token that is only the "Bearer " prefix would send "Authorization: Bearer " with nothing
// after it, which is never what was meant.
func TestToOptionsBearerTokenRejectsEmptyAfterPrefix(t *testing.T) {
	for _, token := range []string{"Bearer ", "bearer", "Bearer    "} {
		c := Default()
		c.Crawl.BearerToken = token
		_, err := c.ToOptions()
		if err == nil {
			t.Fatalf("ToOptions(%q) succeeded, want an error", token)
		}
		if !strings.Contains(err.Error(), "bearer_token") {
			t.Errorf("error %q doesn't name bearer_token", err)
		}
	}
}

// Credentials reach errors that travel to the CLI, MCP results and web API responses, so no
// config error may quote the secret it is rejecting.
func TestToOptionsErrorsNeverEchoSecrets(t *testing.T) {
	const secret = "s3cr3t-value"
	cases := []struct {
		name string
		set  func(*Config)
	}{
		{"basic_auth without a colon", func(c *Config) { c.Crawl.BasicAuth = secret }},
		{"proxy with credentials, bad scheme", func(c *Config) { c.Crawl.Proxy = "ftp://user:" + secret + "@proxy.example:21" }},
		{"proxy with credentials, no host", func(c *Config) { c.Crawl.Proxy = "http://user:" + secret + "@" }},
		{"proxy with credentials, unparsable", func(c *Config) { c.Crawl.Proxy = "http://user:" + secret + "@proxy.example:notaport" }},
		{"proxy in the pool", func(c *Config) { c.Crawl.Proxies = []string{"ftp://user:" + secret + "@proxy.example:21"} }},
		{"inline header without a colon", func(c *Config) { c.Crawl.Headers = []string{"Authorization " + secret} }},
		{"inline header with a bad name", func(c *Config) { c.Crawl.Headers = []string{"X Bad " + secret + ": v"} }},
		{"inline header with a bad value", func(c *Config) { c.Crawl.Headers = []string{"X-Token: " + secret + "\r\nX-Evil: 1"} }},
		{"inline reserved header", func(c *Config) { c.Crawl.Headers = []string{"Cookie: " + secret} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.set(&c)
			_, err := c.ToOptions()
			if err == nil {
				t.Fatal("ToOptions succeeded, want an error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error %q leaks the secret", err)
			}
		})
	}
}

func TestToOptionsHeaderErrorNamesTheLineNumber(t *testing.T) {
	c := Default()
	c.Crawl.Headers = []string{"X-One: 1", "no colon here"}
	_, err := c.ToOptions()
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("err = %v, want it to name line 2", err)
	}
}
