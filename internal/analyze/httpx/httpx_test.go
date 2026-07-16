package httpx

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func htmlPage(t *testing.T, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{FinalURL: "https://example.com/", StatusCode: 200, ContentType: "text/html", Doc: doc}
}

func hasCode(issues []analyze.Issue, code string) bool {
	for _, iss := range issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}

func TestMixedContentDetectsRealSubresources(t *testing.T) {
	tags := []string{
		`<img src="http://insecure.example/x.png">`,
		`<script src="http://insecure.example/x.js"></script>`,
		`<iframe src="http://insecure.example/embed"></iframe>`,
		`<video src="http://insecure.example/v.mp4"></video>`,
		`<audio src="http://insecure.example/a.mp3"></audio>`,
		`<video><source src="http://insecure.example/v2.mp4"></video>`,
		`<embed src="http://insecure.example/e.swf">`,
		`<object data="http://insecure.example/o.swf"></object>`,
		`<input type="image" src="http://insecure.example/i.png">`,
		`<link rel="stylesheet" href="http://insecure.example/s.css">`,
	}
	for _, tag := range tags {
		t.Run(tag, func(t *testing.T) {
			p := htmlPage(t, `<html><head></head><body>`+tag+`</body></html>`)
			result := &crawler.Result{Pages: []*crawler.Page{p}}
			if !hasCode(New().Analyze(context.Background(), result), "http-mixed-content") {
				t.Errorf("expected http-mixed-content for %s", tag)
			}
		})
	}
}

func TestMixedContentIgnoresNonLoadingLinkRels(t *testing.T) {
	tags := []string{
		`<link rel="alternate" type="application/rss+xml" href="http://insecure.example/feed.xml">`,
		`<link rel="canonical" href="http://insecure.example/canonical">`,
		`<link rel="amphtml" href="http://insecure.example/amp">`,
		`<link rel="dns-prefetch" href="http://insecure.example/">`,
		`<link rel="next" href="http://insecure.example/page/2">`,
	}
	for _, tag := range tags {
		t.Run(tag, func(t *testing.T) {
			p := htmlPage(t, `<html><head>`+tag+`</head><body></body></html>`)
			result := &crawler.Result{Pages: []*crawler.Page{p}}
			if hasCode(New().Analyze(context.Background(), result), "http-mixed-content") {
				t.Errorf("did not expect http-mixed-content for %s", tag)
			}
		})
	}
}

func TestTruncatedBodyReported(t *testing.T) {
	result := &crawler.Result{Pages: []*crawler.Page{
		{RequestedURL: "https://example.com/big", StatusCode: 200, Truncated: true},
	}}
	gotTruncated := false
	for _, iss := range New().Analyze(context.Background(), result) {
		if iss.Code == "http-body-truncated" {
			gotTruncated = true
		}
	}
	if !gotTruncated {
		t.Error("expected http-body-truncated for a page with Truncated=true")
	}
}

func TestClientErrorReportsReferrer(t *testing.T) {
	result := &crawler.Result{Pages: []*crawler.Page{
		{RequestedURL: "https://example.com/missing", StatusCode: 404, Referrer: "https://example.com/blog"},
	}}

	var foundOn any
	gotClientError := false
	for _, iss := range New().Analyze(context.Background(), result) {
		if iss.Code == "http-client-error" {
			gotClientError = true
			foundOn = iss.Data["found_on"]
		}
	}
	if !gotClientError {
		t.Fatal("expected a client-error issue for the 404 page")
	}
	if foundOn != "https://example.com/blog" {
		t.Errorf("client-error found_on = %v, want %q", foundOn, "https://example.com/blog")
	}
}

func TestTrailingSlashRedirectIsNotLoop(t *testing.T) {
	// WordPress's canonical "/foo" -> "/foo/" 301 is a benign single hop and
	// must not be mistaken for a redirect loop.
	const reqURL = "https://shop.example.com/product/green-pad"
	const finalURL = reqURL + "/"
	result := &crawler.Result{Pages: []*crawler.Page{{
		RequestedURL: reqURL,
		FinalURL:     finalURL,
		StatusCode:   200,
		Redirects:    []crawler.Redirect{{From: reqURL, To: finalURL, Status: 301}},
	}}}

	var codes []string
	for _, iss := range New().Analyze(context.Background(), result) {
		codes = append(codes, iss.Code)
		if iss.Code == "http-redirect-loop" {
			t.Errorf("trailing-slash 301 reported as redirect-loop; chain=%v", iss.Data["chain"])
		}
		if iss.Code == "http-redirect-chain" {
			t.Errorf("single-hop trailing-slash 301 reported as redirect-chain; chain=%v", iss.Data["chain"])
		}
	}

	gotRedirect := false
	for _, c := range codes {
		if c == "http-redirect" {
			gotRedirect = true
		}
	}
	if !gotRedirect {
		t.Errorf("expected an informational redirect issue, got codes %v", codes)
	}
}

func TestRealRedirectLoopDetected(t *testing.T) {
	// An actual cycle (A -> B -> A -> B) exhausts the fetcher's hop budget before it can
	// report success, so the fetcher sets Err = "too many redirects" — this is the state
	// a real crawl produces for a genuine loop, unlike an artificially clean Redirects-only
	// fixture with no Err.
	const a = "https://example.com/a"
	const b = "https://example.com/b"
	result := &crawler.Result{Pages: []*crawler.Page{{
		RequestedURL: a,
		FinalURL:     b,
		Err:          "too many redirects",
		Redirects: []crawler.Redirect{
			{From: a, To: b, Status: 301},
			{From: b, To: a, Status: 301},
			{From: a, To: b, Status: 301},
		},
	}}}

	gotLoop, gotFetchError := false, false
	for _, iss := range New().Analyze(context.Background(), result) {
		switch iss.Code {
		case "http-redirect-loop":
			gotLoop = true
		case "http-fetch-error":
			gotFetchError = true
		}
	}
	if !gotLoop {
		t.Error("expected redirect-loop for an A -> B -> A cycle, even though the fetcher reports Err")
	}
	if gotFetchError {
		t.Error("http-fetch-error should be suppressed when a more specific redirect-loop was found")
	}
}

func TestTooManyRedirectsWithoutLoopStillReportsFetchError(t *testing.T) {
	// A long, non-cyclic redirect chain that exhausts the hop budget without ever
	// repeating a URL is not a loop — it should still surface as a generic fetch error.
	const base = "https://example.com/hop"
	var redirects []crawler.Redirect
	for i := 0; i < 11; i++ {
		redirects = append(redirects, crawler.Redirect{
			From:   fmt.Sprintf("%s%d", base, i),
			To:     fmt.Sprintf("%s%d", base, i+1),
			Status: 301,
		})
	}
	result := &crawler.Result{Pages: []*crawler.Page{{
		RequestedURL: base + "0",
		FinalURL:     fmt.Sprintf("%s%d", base, len(redirects)),
		Err:          "too many redirects",
		Redirects:    redirects,
	}}}

	gotLoop, gotFetchError := false, false
	for _, iss := range New().Analyze(context.Background(), result) {
		switch iss.Code {
		case "http-redirect-loop":
			gotLoop = true
		case "http-fetch-error":
			gotFetchError = true
		}
	}
	if gotLoop {
		t.Error("non-cyclic chain incorrectly reported as a redirect loop")
	}
	if !gotFetchError {
		t.Error("expected http-fetch-error for a chain that exhausted the hop budget without looping")
	}
}

func TestErrorWithoutReferrerOmitsFoundOn(t *testing.T) {
	result := &crawler.Result{Pages: []*crawler.Page{
		{RequestedURL: "https://example.com/missing", StatusCode: 404},
	}}

	for _, iss := range New().Analyze(context.Background(), result) {
		if iss.Code == "http-client-error" {
			if _, ok := iss.Data["found_on"]; ok {
				t.Error("found_on should be absent when the page has no referrer")
			}
		}
	}
}
