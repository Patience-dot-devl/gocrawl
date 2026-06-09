package structured_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
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

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func TestStructuredExtractsTypes(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Organization","name":"Acme"}
	</script></head><body></body></html>`)
	issues := structured.New().Analyze(context.Background(), res)
	is, ok := find(issues, "structured-data")
	if !ok {
		t.Fatal("expected structured-data issue")
	}
	types, _ := is.Data["types"].([]string)
	if len(types) != 1 || types[0] != "Organization" {
		t.Errorf("expected [Organization], got %v", types)
	}
}

func TestStructuredInvalidJSON(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">{ not json }</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "invalid-jsonld"); !ok {
		t.Error("expected invalid-jsonld issue")
	}
}

func TestStructuredGraph(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@graph":[{"@type":"WebSite"},{"@type":"BreadcrumbList"}]}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-data")
	if !ok {
		t.Fatal("expected structured-data issue")
	}
	types, _ := is.Data["types"].([]string)
	if len(types) != 2 {
		t.Errorf("expected 2 types from @graph, got %v", types)
	}
}
