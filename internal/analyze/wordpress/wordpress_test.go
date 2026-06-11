package wordpress_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/wordpress"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// fakeFetcher serves canned responses keyed by URL; unknown URLs return 404.
type fakeFetcher struct{ pages map[string]*crawler.Page }

func (f fakeFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Page, error) {
	if p, ok := f.pages[rawURL]; ok {
		return p, nil
	}
	return &crawler.Page{RequestedURL: rawURL, FinalURL: rawURL, StatusCode: 404}, nil
}

func doc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return d
}

// page builds an HTML page with the given final URL and body.
func page(t *testing.T, finalURL, html string) *crawler.Page {
	return &crawler.Page{FinalURL: finalURL, StatusCode: 200, ContentType: "text/html", Body: []byte(html), Doc: doc(t, html)}
}

// result wraps pages with an example.com seed so the site base resolves.
func result(pages ...*crawler.Page) *crawler.Result {
	return &crawler.Result{Seed: "https://example.com/", Pages: pages}
}

func run(t *testing.T, res *crawler.Result) []analyze.Issue {
	t.Helper()
	return wordpress.New(fakeFetcher{}).Analyze(context.Background(), res)
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

const wpHome = `<html><head>
<meta name="generator" content="WordPress 6.4.2">
<link rel="stylesheet" href="/wp-content/themes/twentytwentyfour/style.css">
</head><body><p>Hello</p></body></html>`

func TestNotWordPressIsSilent(t *testing.T) {
	html := `<html><head><meta name="generator" content="Drupal 10"></head><body><p>Hi</p></body></html>`
	if issues := run(t, result(page(t, "https://example.com/", html))); len(issues) != 0 {
		t.Errorf("non-WordPress site should produce no issues, got %d", len(issues))
	}
}

func TestDetectViaGeneratorAndVersion(t *testing.T) {
	issues := run(t, result(page(t, "https://example.com/", wpHome)))
	if _, ok := find(issues, "wp-detected"); !ok {
		t.Fatal("expected wp-detected")
	}
	is, ok := find(issues, "wp-version-exposed")
	if !ok {
		t.Fatal("expected wp-version-exposed when generator discloses a version")
	}
	if is.Data["version"] != "6.4.2" {
		t.Errorf("expected version 6.4.2, got %v", is.Data["version"])
	}
}

func TestDetectViaHeaderOnly(t *testing.T) {
	// No generator tag and no wp paths in markup, but an X-Pingback header.
	html := `<html><head></head><body><p>Hi</p></body></html>`
	p := page(t, "https://example.com/", html)
	p.Header = http.Header{"X-Pingback": {"https://example.com/xmlrpc.php"}}
	issues := run(t, result(p))
	if _, ok := find(issues, "wp-detected"); !ok {
		t.Error("expected wp-detected via X-Pingback header")
	}
	if _, ok := find(issues, "wp-version-exposed"); ok {
		t.Error("wp-version-exposed should not fire without a disclosed version")
	}
}

func TestEmojiJqueryAndPlugins(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<html><head><meta name="generator" content="WordPress">`)
	b.WriteString(`<script src="/wp-includes/js/wp-emoji-release.min.js"></script>`)
	b.WriteString(`<script src="/wp-includes/js/jquery/jquery-migrate.min.js"></script>`)
	for i := 0; i < 11; i++ {
		b.WriteString(`<link rel="stylesheet" href="/wp-content/plugins/plugin-` + string(rune('a'+i)) + `/style.css">`)
	}
	b.WriteString(`</head><body><p>Hi</p></body></html>`)

	issues := run(t, result(page(t, "https://example.com/", b.String())))
	if _, ok := find(issues, "wp-emoji-enabled"); !ok {
		t.Error("expected wp-emoji-enabled")
	}
	if _, ok := find(issues, "wp-jquery-migrate"); !ok {
		t.Error("expected wp-jquery-migrate")
	}
	is, ok := find(issues, "wp-many-plugin-assets")
	if !ok {
		t.Fatal("expected wp-many-plugin-assets with 11 distinct plugins")
	}
	if c, _ := is.Data["plugin_count"].(int); c != 11 {
		t.Errorf("expected plugin_count 11, got %v", is.Data["plugin_count"])
	}
}

func TestSeoPluginDetectedSuppressesMissing(t *testing.T) {
	html := strings.Replace(wpHome, "<p>Hello</p>",
		`<!-- This site is optimized with the Yoast SEO plugin --><p>Hello</p>`, 1)
	issues := run(t, result(page(t, "https://example.com/", html)))
	det, _ := find(issues, "wp-detected")
	if det.Data["seo_plugin"] != "Yoast SEO" {
		t.Errorf("expected seo_plugin Yoast SEO, got %v", det.Data["seo_plugin"])
	}
	if _, ok := find(issues, "wp-no-seo-plugin"); ok {
		t.Error("wp-no-seo-plugin should not fire when an SEO plugin is detected")
	}
}

func TestNoSeoPluginAndDefaultTagline(t *testing.T) {
	html := strings.Replace(wpHome, "<p>Hello</p>",
		`<p>Just another WordPress site</p>`, 1)
	issues := run(t, result(page(t, "https://example.com/", html)))
	if _, ok := find(issues, "wp-no-seo-plugin"); !ok {
		t.Error("expected wp-no-seo-plugin")
	}
	if _, ok := find(issues, "wp-default-tagline"); !ok {
		t.Error("expected wp-default-tagline")
	}
}

func TestUglyPermalinkPerPage(t *testing.T) {
	pretty := page(t, "https://example.com/", wpHome)
	ugly := page(t, "https://example.com/?p=42", wpHome)
	issues := run(t, result(pretty, ugly))
	is, ok := find(issues, "wp-ugly-permalink")
	if !ok {
		t.Fatal("expected wp-ugly-permalink for ?p=42")
	}
	if is.URL != "https://example.com/?p=42" {
		t.Errorf("expected ugly-permalink on the ?p= page, got %s", is.URL)
	}
	if is.Data["param"] != "p" {
		t.Errorf("expected param p, got %v", is.Data["param"])
	}
}

// --- security probes (opt-in) ---

func runProbed(ff fakeFetcher, res *crawler.Result) []analyze.Issue {
	return wordpress.New(ff, wordpress.WithSecurityProbes(true)).Analyze(context.Background(), res)
}

func TestProbesOffByDefault(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/xmlrpc.php": {StatusCode: 405, Body: []byte("XML-RPC server accepts POST requests only.")},
	}}
	// Default analyzer (no probes) must not emit any probe finding even when the endpoint is live.
	issues := wordpress.New(ff).Analyze(context.Background(), result(page(t, "https://example.com/", wpHome)))
	if _, ok := find(issues, "wp-xmlrpc-enabled"); ok {
		t.Error("xmlrpc probe must not run without WithSecurityProbes")
	}
}

func TestSecurityProbes(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/xmlrpc.php": {
			FinalURL: "https://example.com/xmlrpc.php", StatusCode: 405,
			Body: []byte("XML-RPC server accepts POST requests only."),
		},
		"https://example.com/wp-json/wp/v2/users": {
			FinalURL: "https://example.com/wp-json/wp/v2/users", StatusCode: 200,
			Body: []byte(`[{"id":1,"name":"Admin","slug":"admin"},{"id":2,"slug":"editor"}]`),
		},
		"https://example.com/?author=1": {
			FinalURL: "https://example.com/author/admin/", StatusCode: 200, Body: []byte("<html></html>"),
		},
		"https://example.com/wp-content/uploads/": {
			FinalURL: "https://example.com/wp-content/uploads/", StatusCode: 200,
			Body: []byte("<title>Index of /wp-content/uploads</title>"),
		},
		"https://example.com/readme.html": {
			FinalURL: "https://example.com/readme.html", StatusCode: 200,
			Body: []byte("<h1>WordPress</h1><p>Version 6.4.2</p>"),
		},
	}}
	issues := runProbed(ff, result(page(t, "https://example.com/", wpHome)))

	if _, ok := find(issues, "wp-xmlrpc-enabled"); !ok {
		t.Error("expected wp-xmlrpc-enabled")
	}
	if is, ok := find(issues, "wp-user-enumeration-rest"); !ok {
		t.Error("expected wp-user-enumeration-rest")
	} else if c, _ := is.Data["count"].(int); c != 2 {
		t.Errorf("expected 2 usernames, got %v", is.Data["count"])
	}
	if is, ok := find(issues, "wp-user-enumeration-author"); !ok {
		t.Error("expected wp-user-enumeration-author")
	} else if is.Data["username"] != "admin" {
		t.Errorf("expected username admin, got %v", is.Data["username"])
	}
	if _, ok := find(issues, "wp-directory-listing"); !ok {
		t.Error("expected wp-directory-listing")
	}
	if is, ok := find(issues, "wp-readme-exposed"); !ok {
		t.Error("expected wp-readme-exposed")
	} else if is.Data["version"] != "6.4.2" {
		t.Errorf("expected readme version 6.4.2, got %v", is.Data["version"])
	}
}

func TestProbesQuietWhenEndpointsAbsent(t *testing.T) {
	// All probe URLs 404 (fakeFetcher default). No probe finding should fire.
	issues := runProbed(fakeFetcher{}, result(page(t, "https://example.com/", wpHome)))
	for _, code := range []string{"wp-xmlrpc-enabled", "wp-user-enumeration-rest", "wp-user-enumeration-author", "wp-directory-listing", "wp-readme-exposed"} {
		if _, ok := find(issues, code); ok {
			t.Errorf("%s should not fire when the endpoint is absent", code)
		}
	}
}
