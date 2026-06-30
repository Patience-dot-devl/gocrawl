package httpx

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

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
	// An actual cycle (A -> B -> A) must still be flagged.
	const a = "https://example.com/a"
	const b = "https://example.com/b"
	result := &crawler.Result{Pages: []*crawler.Page{{
		RequestedURL: a,
		FinalURL:     a,
		Redirects: []crawler.Redirect{
			{From: a, To: b, Status: 301},
			{From: b, To: a, Status: 301},
		},
	}}}

	gotLoop := false
	for _, iss := range New().Analyze(context.Background(), result) {
		if iss.Code == "http-redirect-loop" {
			gotLoop = true
		}
	}
	if !gotLoop {
		t.Error("expected redirect-loop for an A -> B -> A cycle")
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
