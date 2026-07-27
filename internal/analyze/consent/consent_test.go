package consent_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/consent"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// gtagLoader is the standard Google tag snippet, used to give pages a tracker to consent to.
const gtagLoader = `<script async src="https://www.googletagmanager.com/gtag/js?id=G-ABC1234567"></script>`

// consentModeV2 is a correct Consent Mode v2 default: everything denied, both v2 signals
// present, and a wait for the CMP.
const consentModeV2 = `<script>
gtag('consent', 'default', {
  'ad_storage': 'denied',
  'analytics_storage': 'denied',
  'ad_user_data': 'denied',
  'ad_personalization': 'denied',
  'wait_for_update': 500
});
</script>`

// page builds an HTML 200 page from a <head> fragment.
func page(t *testing.T, url, head string) *crawler.Page {
	t.Helper()
	html := "<html><head>" + head + "</head><body><p>hi</p></body></html>"
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{
		RequestedURL: url, FinalURL: url, StatusCode: 200,
		ContentType: "text/html", Doc: doc, Header: http.Header{},
	}
}

func run(pages ...*crawler.Page) []analyze.Issue {
	return consent.New().Analyze(context.Background(), &crawler.Result{Pages: pages})
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func codeList(issues []analyze.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, is := range issues {
		out = append(out, is.Code)
	}
	return out
}

func mustFind(t *testing.T, issues []analyze.Issue, code string, want analyze.Severity) analyze.Issue {
	t.Helper()
	got, ok := find(issues, code)
	if !ok {
		t.Fatalf("expected %q, got %v", code, codeList(issues))
	}
	if got.Severity != want {
		t.Errorf("%s severity = %q, want %q", code, got.Severity, want)
	}
	return got
}

func mustNotFind(t *testing.T, issues []analyze.Issue, code string) {
	t.Helper()
	if got, ok := find(issues, code); ok {
		t.Errorf("unexpected %q: %s", code, got.Message)
	}
}

func TestDetectsCMP(t *testing.T) {
	tests := []struct{ name, markup, want string }{
		{"Cookiebot", `<script src="https://consent.cookiebot.com/uc.js"></script>`, "Cookiebot"},
		{"OneTrust", `<script src="https://cdn.cookielaw.org/scripttemplates/otSDKStub.js"></script>`, "OneTrust"},
		{"Usercentrics", `<script src="https://app.usercentrics.eu/browser-ui/latest/loader.js"></script>`, "Usercentrics"},
		{"Complianz", `<script>var cmplz_settings = {};</script>`, "Complianz"},
		{"generic TCF", `<script>window.__tcfapi('addEventListener', 2, cb);</script>`, "IAB TCF CMP (vendor not identified)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := run(page(t, "https://example.com/", gtagLoader+tc.markup))
			got := mustFind(t, issues, "consent-cmp-detected", analyze.Info)
			if got.Data["cmp"] != tc.want {
				t.Errorf("cmp = %v, want %q", got.Data["cmp"], tc.want)
			}
			mustNotFind(t, issues, "consent-no-cmp")
		})
	}
}

// TestCMPNotDetectedFromProse is a regression test for a real false positive: Cookiebot's own
// marketing site was reported as running OneTrust, because vendor names are matched as
// substrings and their comparison pages name every competitor. Detection must read code and
// attributes, never visible copy.
func TestCMPNotDetectedFromProse(t *testing.T) {
	// The inline JSON is the shape that actually broke this: cookiebot.com ships its nav menu
	// as a script blob naming every competitor, so restricting to code alone was not enough —
	// the signals themselves had to become integration-shaped.
	html := `<html><head>` + gtagLoader + `
	  <script>window.__nav = {"items":[
	    {"title":"Cookiebot vs OneTrust","url":"/en/onetrust-alternative/"},
	    {"title":"Cookiebot vs Didomi","url":"/en/didomi-alternative/"},
	    {"title":"Cookiebot vs Sourcepoint","url":"/en/sourcepoint-alternative/"},
	    {"title":"Cookiebot vs Complianz","url":"/en/complianz-alternative/"}]};</script>
	</head><body>
	  <h1>The best OneTrust alternative</h1>
	  <p>Switching from Usercentrics or Didomi? Our CMP beats them all.</p>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	p := &crawler.Page{
		RequestedURL: "https://example.com/", FinalURL: "https://example.com/",
		StatusCode: 200, ContentType: "text/html", Doc: doc, Header: http.Header{},
	}

	issues := run(p)
	if got, ok := find(issues, "consent-cmp-detected"); ok {
		t.Errorf("CMP %v detected from marketing copy alone", got.Data["cmp"])
	}
	mustFind(t, issues, "consent-no-cmp", analyze.Warning)
}

// TestCMPDetectedFromBannerClass covers the CMPs that leave no identifiable script — only a
// class on the container they inject.
func TestCMPDetectedFromBannerClass(t *testing.T) {
	html := `<html><head>` + gtagLoader + `</head><body>
	  <div id="cmplz-cookiebanner-container" class="cmplz-cookiebanner">Accept?</div>
	</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	p := &crawler.Page{
		RequestedURL: "https://example.com/", FinalURL: "https://example.com/",
		StatusCode: 200, ContentType: "text/html", Doc: doc, Header: http.Header{},
	}

	got := mustFind(t, run(p), "consent-cmp-detected", analyze.Info)
	if got.Data["cmp"] != "Complianz" {
		t.Errorf("cmp = %v, want Complianz", got.Data["cmp"])
	}
}

// TestCMPDetectedFromPreconnect covers first-party-proxied CMPs, whose only static trace is a
// resource hint (as on theguardian.com, which self-hosts Sourcepoint).
func TestCMPDetectedFromPreconnect(t *testing.T) {
	head := gtagLoader + `<link rel="preconnect" href="https://sourcepoint.example.com">`
	got := mustFind(t, run(page(t, "https://example.com/", head)), "consent-cmp-detected", analyze.Info)
	if got.Data["cmp"] != "Sourcepoint" {
		t.Errorf("cmp = %v, want Sourcepoint", got.Data["cmp"])
	}
}

func TestNoCMPWithTrackers(t *testing.T) {
	issues := run(page(t, "https://example.com/", gtagLoader))
	mustFind(t, issues, "consent-no-cmp", analyze.Warning)
}

// TestNoCMPNotFlaggedWithoutTrackers keeps the finding honest: a page with nothing to consent
// to needs no consent banner, and demanding one would be noise on brochure sites.
func TestNoCMPNotFlaggedWithoutTrackers(t *testing.T) {
	issues := run(page(t, "https://example.com/", `<title>t</title>`))
	mustNotFind(t, issues, "consent-no-cmp")
}

func TestConsentModeV2CleanConfiguration(t *testing.T) {
	head := consentModeV2 + gtagLoader + `<script src="https://consent.cookiebot.com/uc.js"></script>`
	issues := run(page(t, "https://example.com/", head))

	for _, code := range []string{
		"consent-mode-v1-only", "consent-mode-default-granted",
		"consent-mode-no-wait-for-update", "consent-mode-after-tags",
	} {
		mustNotFind(t, issues, code)
	}
}

func TestConsentModeV1Only(t *testing.T) {
	head := `<script>gtag('consent','default',{'ad_storage':'denied','analytics_storage':'denied','wait_for_update':500});</script>` + gtagLoader
	issues := run(page(t, "https://example.com/", head))

	got := mustFind(t, issues, "consent-mode-v1-only", analyze.Warning)
	missing, _ := got.Data["missing"].([]string)
	if len(missing) != 2 {
		t.Errorf("missing = %v, want both v2 signals", got.Data["missing"])
	}
}

func TestConsentModeDefaultGranted(t *testing.T) {
	head := `<script>gtag('consent','default',{'ad_storage':'granted','analytics_storage':'granted','ad_user_data':'granted','ad_personalization':'granted','wait_for_update':500});</script>` + gtagLoader
	issues := run(page(t, "https://example.com/", head))

	got := mustFind(t, issues, "consent-mode-default-granted", analyze.Error)
	granted, _ := got.Data["granted"].([]string)
	if len(granted) != 4 {
		t.Errorf("granted = %v, want all four gated signals", got.Data["granted"])
	}
}

// TestConsentModeRegionalDefaultsAllowed covers the legitimate pattern of granting by default
// outside the EEA and denying inside it: `region`-scoped defaults must not be flagged.
func TestConsentModeRegionalDefaultsAllowed(t *testing.T) {
	head := `<script>
gtag('consent','default',{'ad_storage':'denied','analytics_storage':'denied','ad_user_data':'denied','ad_personalization':'denied','region':['ES','DE','FR'],'wait_for_update':500});
gtag('consent','default',{'ad_storage':'granted','analytics_storage':'granted','ad_user_data':'granted','ad_personalization':'granted','wait_for_update':500});
</script>` + gtagLoader
	issues := run(page(t, "https://example.com/", head))

	mustNotFind(t, issues, "consent-mode-default-granted")
}

func TestConsentModeMissingWaitForUpdate(t *testing.T) {
	head := `<script>gtag('consent','default',{'ad_storage':'denied','analytics_storage':'denied','ad_user_data':'denied','ad_personalization':'denied'});</script>` + gtagLoader
	issues := run(page(t, "https://example.com/", head))

	mustFind(t, issues, "consent-mode-no-wait-for-update", analyze.Info)
}

// TestConsentModeAfterTags is the ordering contract: identical configuration, only the
// position of the default relative to the loader differs.
func TestConsentModeAfterTags(t *testing.T) {
	late := run(page(t, "https://example.com/", gtagLoader+consentModeV2))
	mustFind(t, late, "consent-mode-after-tags", analyze.Warning)

	early := run(page(t, "https://example.com/", consentModeV2+gtagLoader))
	mustNotFind(t, early, "consent-mode-after-tags")
}

// TestConsentModeViaDataLayerPush covers the raw form the gtag shim compiles to, which sites
// using GTM without the gtag helper emit directly.
func TestConsentModeViaDataLayerPush(t *testing.T) {
	head := `<script>window.dataLayer=window.dataLayer||[];dataLayer.push(['consent','default',{'ad_storage':'granted','analytics_storage':'granted','wait_for_update':500}]);</script>` + gtagLoader
	issues := run(page(t, "https://example.com/", head))

	mustFind(t, issues, "consent-mode-default-granted", analyze.Error)
}

// TestConsentModeSilentWhenAbsent defers to the datalayer analyzer, which owns the
// "Consent Mode not wired at all" finding. Two analyzers reporting it would be duplication.
func TestConsentModeSilentWhenAbsent(t *testing.T) {
	issues := run(page(t, "https://example.com/", gtagLoader))
	for _, code := range []string{"consent-mode-v1-only", "consent-mode-default-granted", "consent-mode-no-wait-for-update", "consent-mode-after-tags"} {
		mustNotFind(t, issues, code)
	}
}

// TestReportsOncePerHost is the aggregation contract: consent configuration lives in the
// shared template, so a multi-page crawl must not repeat the same finding per page.
func TestReportsOncePerHost(t *testing.T) {
	issues := run(
		page(t, "https://example.com/", gtagLoader),
		page(t, "https://example.com/about", gtagLoader),
		page(t, "https://example.com/contact", gtagLoader),
	)
	n := 0
	for _, is := range issues {
		if is.Code == "consent-no-cmp" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("consent-no-cmp emitted %d times across 3 pages of one host, want 1", n)
	}
}

// TestCMPOnAnyPageCountsForHost covers a banner injected only on some pages: the host has a
// CMP, so the whole host should not be reported as lacking one.
func TestCMPOnAnyPageCountsForHost(t *testing.T) {
	issues := run(
		page(t, "https://example.com/", gtagLoader),
		page(t, "https://example.com/about", gtagLoader+`<script src="https://cdn.cookielaw.org/scripttemplates/otSDKStub.js"></script>`),
	)
	mustFind(t, issues, "consent-cmp-detected", analyze.Info)
	mustNotFind(t, issues, "consent-no-cmp")
}
