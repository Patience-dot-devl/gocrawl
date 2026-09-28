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
	if _, err := c.ToOptions(); err == nil || !strings.Contains(err.Error(), "header_profile") {
		t.Errorf("err = %v, want a header_profile error", err)
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
