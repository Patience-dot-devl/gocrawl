package security_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/security"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func htmlPage(t *testing.T, html string, hdr http.Header) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{FinalURL: "https://example.com/", StatusCode: 200, ContentType: "text/html", Doc: doc, Header: hdr}
}

func codes(issues []analyze.Issue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		out[is.Code] = true
	}
	return out
}

func TestSecurityFlagsMissingHeadersAndInsecureForm(t *testing.T) {
	p := htmlPage(t, `<html><body><form action="http://example.com/submit"></form></body></html>`, http.Header{})
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(security.New().Analyze(context.Background(), res))

	for _, want := range []string{"security-missing-hsts", "security-missing-csp", "security-missing-x-content-type-options", "security-insecure-form"} {
		if !got[want] {
			t.Errorf("expected issue %q, not found", want)
		}
	}
}

func TestSecurityCleanPage(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("Strict-Transport-Security", "max-age=63072000")
	hdr.Set("Content-Security-Policy", "default-src 'self'")
	hdr.Set("X-Content-Type-Options", "nosniff")
	p := htmlPage(t, `<html><body><form action="https://example.com/submit"></form></body></html>`, hdr)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(security.New().Analyze(context.Background(), res))

	for _, unwanted := range []string{"security-missing-hsts", "security-missing-csp", "security-missing-x-content-type-options", "security-insecure-form"} {
		if got[unwanted] {
			t.Errorf("did not expect issue %q on a clean page", unwanted)
		}
	}
}

func TestSecurityNilHeaderStillChecksForms(t *testing.T) {
	p := htmlPage(t, `<html><body><form action="http://example.com/submit"></form></body></html>`, nil)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(security.New().Analyze(context.Background(), res))

	if !got["security-insecure-form"] {
		t.Error("expected insecure-form even with nil Header")
	}
	for _, unwanted := range []string{"security-missing-hsts", "security-missing-csp", "security-missing-x-content-type-options"} {
		if got[unwanted] {
			t.Errorf("did not expect header issue %q when Header is nil", unwanted)
		}
	}
}
