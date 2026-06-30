package botwall_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/botwall"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

type pageOpt func(*crawler.Page)

func withStatus(code int) pageOpt { return func(p *crawler.Page) { p.StatusCode = code } }
func withHeader(k, v string) pageOpt {
	return func(p *crawler.Page) {
		if p.Header == nil {
			p.Header = http.Header{}
		}
		p.Header.Add(k, v)
	}
}
func withRequests(urls ...string) pageOpt {
	return func(p *crawler.Page) { p.Render = &crawler.RenderResult{Requests: urls} }
}

func mkPage(t *testing.T, html string, opts ...pageOpt) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	p := &crawler.Page{
		RequestedURL: "https://example.com/x", FinalURL: "https://example.com/x",
		StatusCode: 200, ContentType: "text/html", Body: []byte(html), Doc: doc,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func analyzePage(p *crawler.Page) []analyze.Issue {
	res := &crawler.Result{Seed: "https://example.com", Pages: []*crawler.Page{p}}
	return botwall.New().Analyze(context.Background(), res)
}

func codeFor(issues []analyze.Issue) string {
	for _, is := range issues {
		if is.Code == "botwall-challenge" || is.Code == "botwall-captcha-widget" {
			return is.Code
		}
	}
	return ""
}

// longText returns body content well past the thin-content threshold.
func longText() string {
	return strings.Repeat("Real article content about onboarding checklists for managers. ", 60)
}

func TestRecaptchaChallengePage(t *testing.T) {
	// A thin page that is essentially just a reCAPTCHA → a block, not an embedded widget.
	html := `<html><head><title>Please verify</title></head><body>
		<div class="g-recaptcha"></div>
		<script src="https://www.google.com/recaptcha/api.js"></script></body></html>`
	issues := analyzePage(mkPage(t, html))
	if got := codeFor(issues); got != "botwall-challenge" {
		t.Fatalf("expected bot-challenge, got %q (%+v)", got, issues)
	}
}

func TestCloudflareInterstitial(t *testing.T) {
	html := `<html><head><title>Just a moment...</title></head><body>
		<script src="/cdn-cgi/challenge-platform/h/g/orchestrate/chl_page/v1"></script></body></html>`
	issues := analyzePage(mkPage(t, html, withStatus(403)))
	is := find(issues, "botwall-challenge")
	if is == nil {
		t.Fatalf("expected bot-challenge, got %+v", issues)
	}
	if is.Data["provider"] != "Cloudflare" {
		t.Errorf("expected provider Cloudflare, got %v", is.Data["provider"])
	}
}

func TestDataDomeOn403(t *testing.T) {
	html := `<html><head><title>Access to this page has been denied</title></head><body>
		<script src="https://geo.captcha-delivery.com/captcha/"></script></body></html>`
	if got := codeFor(analyzePage(mkPage(t, html, withStatus(403)))); got != "botwall-challenge" {
		t.Fatalf("expected bot-challenge for DataDome, got %q", got)
	}
}

func TestRecaptchaWidgetOnRealPageIsNotABlock(t *testing.T) {
	// reCAPTCHA embedded on a full content page (e.g. a contact form) → info, not a block.
	html := `<html><head><title>Contact us</title></head><body><main><h1>Contact</h1><p>` +
		longText() + `</p><form><div class="g-recaptcha"></div></form></main></body></html>`
	issues := analyzePage(mkPage(t, html))
	if got := codeFor(issues); got != "botwall-captcha-widget" {
		t.Fatalf("expected captcha-widget (info), got %q (%+v)", got, issues)
	}
	for _, is := range issues {
		if is.Code == "botwall-challenge" {
			t.Error("a legit embedded reCAPTCHA must not be flagged as a block")
		}
	}
}

func TestCleanPageHasNoFinding(t *testing.T) {
	html := `<html><head><title>Home</title></head><body><main><h1>Hi</h1><p>` + longText() + `</p></main></body></html>`
	if got := codeFor(analyzePage(mkPage(t, html))); got != "" {
		t.Fatalf("expected no botwall finding on a clean page, got %q", got)
	}
}

func TestTurnstileViaHeadlessRequests(t *testing.T) {
	// Headless shell DOM: the challenge script only shows up in captured requests.
	html := `<html><head><title>Just a moment...</title></head><body></body></html>`
	p := mkPage(t, html, withStatus(403),
		withRequests("https://challenges.cloudflare.com/turnstile/v0/api.js"))
	if got := codeFor(analyzePage(p)); got != "botwall-challenge" {
		t.Fatalf("expected bot-challenge from captured Turnstile request, got %q", got)
	}
}

func TestCloudflareViaResponseHeader(t *testing.T) {
	// Cloudflare sets `cf-mitigated: challenge` on challenge responses — detectable from the
	// headers even when the body is an empty JS shell.
	html := `<html><head><title>Just a moment...</title></head><body></body></html>`
	p := mkPage(t, html, withStatus(403), withHeader("Cf-Mitigated", "challenge"))
	if got := codeFor(analyzePage(p)); got != "botwall-challenge" {
		t.Fatalf("expected bot-challenge from cf-mitigated header, got %q", got)
	}
}

func TestUnknownChallengeOnBlockingStatus(t *testing.T) {
	// No known vendor, but a challenge-style title on a 429 → flagged as Unknown.
	html := `<html><head><title>Checking your browser before access</title></head><body>x</body></html>`
	is := find(analyzePage(mkPage(t, html, withStatus(429))), "botwall-challenge")
	if is == nil {
		t.Fatal("expected an Unknown bot-challenge on a blocking status")
	}
	if is.Data["provider"] != "Unknown" {
		t.Errorf("expected provider Unknown, got %v", is.Data["provider"])
	}
}

func find(issues []analyze.Issue, code string) *analyze.Issue {
	for i := range issues {
		if issues[i].Code == code {
			return &issues[i]
		}
	}
	return nil
}
