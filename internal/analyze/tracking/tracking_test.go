package tracking_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/tracking"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func page(t *testing.T, html string) *crawler.Result {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &crawler.Result{Pages: []*crawler.Page{{FinalURL: "https://example.com/", StatusCode: 200, ContentType: "text/html", Doc: doc}}}
}

func run(res *crawler.Result) []analyze.Issue {
	return tracking.New().Analyze(context.Background(), res)
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

// idsForTag pulls the ids recorded for a tag out of a tracking-tags issue.
func idsForTag(is analyze.Issue, tag string) []string {
	tags, _ := is.Data["tags"].([]map[string]any)
	for _, m := range tags {
		if m["tag"] == tag {
			ids, _ := m["ids"].([]string)
			return ids
		}
	}
	return nil
}

func TestGTM(t *testing.T) {
	res := page(t, `<html><head><script>
		(function(w,d,s,l,i){})(window,document,'script','dataLayer','GTM-ABC123');
		// https://www.googletagmanager.com/gtm.js?id=GTM-ABC123
	</script></head><body></body></html>`)
	is, ok := find(run(res), "tracking-tags")
	if !ok {
		t.Fatal("expected tracking-tags")
	}
	ids := idsForTag(is, "Google Tag Manager")
	if len(ids) != 1 || ids[0] != "GTM-ABC123" {
		t.Errorf("GTM ids = %v", ids)
	}
}

func TestGA4(t *testing.T) {
	res := page(t, `<html><head>
		<script async src="https://www.googletagmanager.com/gtag/js?id=G-XXXX1234"></script>
	</head><body></body></html>`)
	is, ok := find(run(res), "tracking-tags")
	if !ok {
		t.Fatal("expected tracking-tags")
	}
	if ids := idsForTag(is, "GA4"); len(ids) != 1 || ids[0] != "G-XXXX1234" {
		t.Errorf("GA4 ids = %v", ids)
	}
}

func TestMetaPixelNoFalseDuplicate(t *testing.T) {
	res := page(t, `<html><head><script>
		!function(f,b,e,v,n,t,s){}(window, document);
		fbq('init', '123456789012345');
		fbq('track', 'PageView');
	</script>
	<noscript><img height="1" width="1" src="https://www.facebook.com/tr?id=123456789012345&ev=PageView"/></noscript>
	</head><body></body></html>`)
	issues := run(res)
	is, ok := find(issues, "tracking-tags")
	if !ok {
		t.Fatal("expected tracking-tags")
	}
	if ids := idsForTag(is, "Meta Pixel"); len(ids) != 1 || ids[0] != "123456789012345" {
		t.Errorf("Meta ids = %v (inline + noscript with same id should collapse to one)", ids)
	}
	if _, dup := find(issues, "duplicate-tracking-tag"); dup {
		t.Error("did not expect duplicate-tracking-tag for one pixel installed via script + noscript")
	}
}

func TestDuplicateGTM(t *testing.T) {
	res := page(t, `<html><head><script>
		dataLayer GTM-AAA111 ... GTM-BBB222
	</script></head><body></body></html>`)
	is, ok := find(run(res), "duplicate-tracking-tag")
	if !ok {
		t.Fatal("expected duplicate-tracking-tag for two GTM containers")
	}
	if is.Severity != analyze.Warning {
		t.Errorf("severity = %v, want warning", is.Severity)
	}
}

func TestNoTags(t *testing.T) {
	res := page(t, `<html><head><title>Plain</title></head><body><p>hi</p></body></html>`)
	if _, ok := find(run(res), "no-tracking-tags"); !ok {
		t.Error("expected no-tracking-tags")
	}
}

// An image filename like IMG-12345 must not be misread as a GA4 "G-12345" measurement ID.
func TestImageFilenameNotGA4(t *testing.T) {
	res := page(t, `<html><head><title>Gallery</title></head><body><img src="https://cdn.example.com/IMG-12345.png"></body></html>`)
	if _, ok := find(run(res), "no-tracking-tags"); !ok {
		t.Error("expected no-tracking-tags; IMG- filename should not register as GA4")
	}
}

func TestMixedGAVersions(t *testing.T) {
	res := page(t, `<html><head>
		<script src="https://www.google-analytics.com/analytics.js"></script>
		<script>ga('create','UA-12345-1','auto'); gtag('config','G-XXXX1234');</script>
	</head><body></body></html>`)
	if _, ok := find(run(res), "mixed-ga-versions"); !ok {
		t.Error("expected mixed-ga-versions")
	}
}
