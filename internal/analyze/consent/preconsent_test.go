package consent_test

import (
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// withJar attaches a headless render's cookie jar to a page.
func withJar(p *crawler.Page, names ...string) *crawler.Page {
	jar := make([]crawler.Cookie, 0, len(names))
	for _, n := range names {
		jar = append(jar, crawler.Cookie{Name: n, Domain: "example.com", Path: "/"})
	}
	p.Render = &crawler.RenderResult{Implemented: true, Cookies: jar}
	return p
}

// withRequests attaches the outbound requests a headless render observed.
func withRequests(p *crawler.Page, urls ...string) *crawler.Page {
	if p.Render == nil {
		p.Render = &crawler.RenderResult{Implemented: true}
	}
	p.Render.Requests = urls
	return p
}

// TestPreConsentTrackingCookieFromBrowserJar is the headline check: the crawl never accepted
// a banner, so a Google Analytics cookie in the jar was set without consent.
func TestPreConsentTrackingCookieFromBrowserJar(t *testing.T) {
	p := withJar(page(t, "https://example.com/", gtagLoader), "_ga", "_ga_ABC1234567", "PHPSESSID")
	issues := run(p)

	got := mustFind(t, issues, "consent-preconsent-tracking-cookie", analyze.Error)
	if got.Data["vendor"] != "Google Analytics" {
		t.Errorf("vendor = %v, want Google Analytics", got.Data["vendor"])
	}
	if got.Data["purpose"] != "analytics" {
		t.Errorf("purpose = %v, want analytics", got.Data["purpose"])
	}
	if src, _ := got.Data["source"].(string); src == "" {
		t.Error("finding does not record which evidence source it used")
	}
}

// TestPreConsentCookieClassification checks the vendor mapping across the tracker families a
// real audit meets, and confirms functional cookies are left alone.
func TestPreConsentCookieClassification(t *testing.T) {
	tracking := map[string]string{
		"_ga":        "Google Analytics",
		"_gcl_au":    "Google Ads",
		"_fbp":       "Meta Pixel",
		"_uetsid":    "Microsoft Advertising",
		"_hjSession": "Hotjar",
		"_clck":      "Microsoft Clarity",
		"IDE":        "Google DoubleClick",
		"_ttp":       "TikTok Pixel",
	}
	for name, vendor := range tracking {
		t.Run(name, func(t *testing.T) {
			issues := run(withJar(page(t, "https://example.com/", gtagLoader), name))
			got := mustFind(t, issues, "consent-preconsent-tracking-cookie", analyze.Error)
			if got.Data["vendor"] != vendor {
				t.Errorf("vendor = %v, want %q", got.Data["vendor"], vendor)
			}
		})
	}

	functional := []string{"PHPSESSID", "JSESSIONID", "csrftoken", "cart_items", "wordpress_logged_in_abc"}
	for _, name := range functional {
		t.Run(name, func(t *testing.T) {
			issues := run(withJar(page(t, "https://example.com/", gtagLoader), name))
			mustNotFind(t, issues, "consent-preconsent-tracking-cookie")
		})
	}
}

// TestConsentStateCookiesExempt covers the carve-out that keeps the finding credible: a CMP
// cannot remember a refusal without storing it, so its own cookie is strictly necessary.
func TestConsentStateCookiesExempt(t *testing.T) {
	for _, name := range []string{"CookieConsent", "OptanonConsent", "cmplz_banner-status", "borlabs-cookie", "euconsent-v2"} {
		t.Run(name, func(t *testing.T) {
			issues := run(withJar(page(t, "https://example.com/", gtagLoader), name))
			mustNotFind(t, issues, "consent-preconsent-tracking-cookie")

			inv := mustFind(t, issues, "consent-cookie-inventory", analyze.Info)
			if n, _ := inv.Data["tracking_count"].(int); n != 0 {
				t.Errorf("tracking_count = %v for a consent-state cookie, want 0", inv.Data["tracking_count"])
			}
		})
	}
}

// TestPreConsentCookieFromSetCookieHeader covers the raw-crawl fallback, and asserts the
// finding says the evidence is partial rather than implying a full jar was inspected.
func TestPreConsentCookieFromSetCookieHeader(t *testing.T) {
	p := page(t, "https://example.com/", gtagLoader)
	p.Header.Add("Set-Cookie", "_ga=GA1.2.1; Path=/; Max-Age=63072000")
	issues := run(p)

	got := mustFind(t, issues, "consent-preconsent-tracking-cookie", analyze.Error)
	src, _ := got.Data["source"].(string)
	if src == "" || !strings.Contains(src, "raw crawl") {
		t.Errorf("source = %q, want it to disclose the raw-crawl limitation", src)
	}
}

// TestBrowserJarPreferredOverHeaders confirms the richer evidence wins when both exist, so a
// headless run isn't reported with the raw crawl's caveat.
func TestBrowserJarPreferredOverHeaders(t *testing.T) {
	p := withJar(page(t, "https://example.com/", gtagLoader), "_fbp")
	p.Header.Add("Set-Cookie", "_ga=GA1.2.1; Path=/")
	issues := run(p)

	got := mustFind(t, issues, "consent-preconsent-tracking-cookie", analyze.Error)
	if got.Data["vendor"] != "Meta Pixel" {
		t.Errorf("vendor = %v, want the browser jar's cookie to win", got.Data["vendor"])
	}
	if src, _ := got.Data["source"].(string); !strings.Contains(src, "headless") {
		t.Errorf("source = %q, want the browser-jar source", src)
	}
}

func TestThirdPartyCookieScope(t *testing.T) {
	p := page(t, "https://example.com/", gtagLoader)
	p.Render = &crawler.RenderResult{Implemented: true, Cookies: []crawler.Cookie{
		{Name: "IDE", Domain: ".doubleclick.net"},
		{Name: "_ga", Domain: ".example.com"},
	}}
	issues := run(p)

	var third, first bool
	for _, is := range issues {
		if is.Code != "consent-preconsent-tracking-cookie" {
			continue
		}
		switch is.Data["cookie"] {
		case "IDE":
			third = is.Data["scope"] == "third-party"
		case "_ga":
			first = is.Data["scope"] == "first-party"
		}
	}
	if !third {
		t.Error("doubleclick.net cookie not marked third-party")
	}
	if !first {
		t.Error("cookie on the crawled domain not marked first-party")
	}
}

// TestPreConsentTrackerRequests covers the strongest evidence the crawl can produce: a call
// to a measurement collector during a render that never answered a banner.
func TestPreConsentTrackerRequests(t *testing.T) {
	p := withRequests(page(t, "https://example.com/", gtagLoader),
		"https://example.com/style.css",
		"https://www.google-analytics.com/g/collect?v=2&tid=G-ABC",
		"https://www.facebook.com/tr?id=123&ev=PageView",
	)
	issues := run(p)

	got := mustFind(t, issues, "consent-preconsent-tracker-request", analyze.Error)
	endpoints, _ := got.Data["endpoints"].([]string)
	if len(endpoints) != 2 {
		t.Errorf("endpoints = %v, want the two tracker hosts (and not the stylesheet)", got.Data["endpoints"])
	}
}

// TestNoTrackerRequestsWhenClean confirms an ordinary asset load isn't mistaken for a beacon.
func TestNoTrackerRequestsWhenClean(t *testing.T) {
	p := withRequests(page(t, "https://example.com/", gtagLoader),
		"https://example.com/app.js", "https://fonts.gstatic.com/s/font.woff2")
	mustNotFind(t, run(p), "consent-preconsent-tracker-request")
}

// TestBeaconCheckSilentInRawMode guards against a false all-clear: a raw crawl never runs the
// tags, so the absence of beacons proves nothing and must not be reported either way.
func TestBeaconCheckSilentInRawMode(t *testing.T) {
	mustNotFind(t, run(page(t, "https://example.com/", gtagLoader)), "consent-preconsent-tracker-request")
}

func TestCookieInventoryCounts(t *testing.T) {
	p := withJar(page(t, "https://example.com/", gtagLoader), "_ga", "_fbp", "PHPSESSID", "cart")
	got := mustFind(t, run(p), "consent-cookie-inventory", analyze.Info)

	if n, _ := got.Data["total"].(int); n != 4 {
		t.Errorf("total = %v, want 4", got.Data["total"])
	}
	if n, _ := got.Data["tracking_count"].(int); n != 2 {
		t.Errorf("tracking_count = %v, want 2", got.Data["tracking_count"])
	}
	if n, _ := got.Data["other_count"].(int); n != 2 {
		t.Errorf("other_count = %v, want 2", got.Data["other_count"])
	}
}

// TestCookieDedupedAcrossPages keeps a shared session cookie from being reported once per
// page of the crawl.
func TestCookieDedupedAcrossPages(t *testing.T) {
	issues := run(
		withJar(page(t, "https://example.com/", gtagLoader), "_ga"),
		withJar(page(t, "https://example.com/about", gtagLoader), "_ga"),
		withJar(page(t, "https://example.com/contact", gtagLoader), "_ga"),
	)
	n := 0
	for _, is := range issues {
		if is.Code == "consent-preconsent-tracking-cookie" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("_ga reported %d times across 3 pages, want 1", n)
	}
}

func TestNoCookiesNoFindings(t *testing.T) {
	issues := run(page(t, "https://example.com/", gtagLoader))
	mustNotFind(t, issues, "consent-preconsent-tracking-cookie")
	mustNotFind(t, issues, "consent-cookie-inventory")
}
