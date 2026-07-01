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
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-invalid-jsonld"); !ok {
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

func TestStructuredMissingRequired(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","image":"x.jpg"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for a Product without name")
	}
	if is.Data["type"] != "Product" {
		t.Errorf("expected type Product in data, got %v", is.Data["type"])
	}
}

func TestStructuredValidProductNoViolation(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","name":"Widget"}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("did not expect structured-missing-required for a complete Product")
	}
}

func TestStructuredMissingRequiredInGraph(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@graph":[{"@type":"Organization"}]}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); !ok {
		t.Error("expected structured-missing-required for an Organization without name inside @graph")
	}
}

func TestStructuredBreadcrumbCandidate(t *testing.T) {
	res := page(t, `<html><body>
		<nav aria-label="breadcrumb"><a href="/">Home</a> &gt; <a href="/shoes">Shoes</a></nav>
	</body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-breadcrumb-candidate")
	if !ok {
		t.Fatal("expected structured-breadcrumb-candidate")
	}
	if is.Data["links"] != 2 {
		t.Errorf("expected 2 links, got %v", is.Data["links"])
	}
}

func TestStructuredBreadcrumbCandidateSuppressedByExistingType(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[]}
	</script></head><body>
		<nav aria-label="breadcrumb"><a href="/">Home</a> &gt; <a href="/shoes">Shoes</a></nav>
	</body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-breadcrumb-candidate"); ok {
		t.Error("did not expect structured-breadcrumb-candidate when BreadcrumbList is already present")
	}
}

func TestStructuredProductCandidate(t *testing.T) {
	res := page(t, `<html><body>
		<h1>Widget</h1><p>Price: $19.99</p><button>Add to cart</button>
	</body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate")
	if !ok {
		t.Fatal("expected structured-product-candidate")
	}
	if is.Data["signal"] != "$19.99" {
		t.Errorf("expected signal $19.99, got %v", is.Data["signal"])
	}
}

func TestStructuredProductCandidateNoCartSignal(t *testing.T) {
	res := page(t, `<html><body><p>This gadget costs $19.99 to make.</p></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate without a cart/buy signal")
	}
}

func TestStructuredArticleCandidate(t *testing.T) {
	words := strings.Repeat("word ", 150)
	res := page(t, `<html><body><article>
		<h1>Title</h1><time datetime="2026-01-01">Jan 1</time><p>`+words+`</p>
	</article></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-article-candidate")
	if !ok {
		t.Fatal("expected structured-article-candidate")
	}
	if w, _ := is.Data["words"].(int); w < 150 {
		t.Errorf("expected >= 150 words, got %v", is.Data["words"])
	}
}

func TestStructuredArticleCandidateShortArticleIgnored(t *testing.T) {
	res := page(t, `<html><body><article><time datetime="2026-01-01">Jan 1</time><p>Too short.</p></article></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-article-candidate"); ok {
		t.Error("did not expect structured-article-candidate for a short article")
	}
}

func TestStructuredVideoCandidate(t *testing.T) {
	res := page(t, `<html><body>
		<iframe src="https://www.youtube.com/embed/abc123"></iframe>
	</body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-video-candidate")
	if !ok {
		t.Fatal("expected structured-video-candidate")
	}
	if is.Data["src"] != "https://www.youtube.com/embed/abc123" {
		t.Errorf("unexpected src %v", is.Data["src"])
	}
}

func TestStructuredVideoCandidateSuppressedByExistingType(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"VideoObject","name":"x","thumbnailUrl":"x.jpg"}
	</script></head><body>
		<iframe src="https://www.youtube.com/embed/abc123"></iframe>
	</body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-video-candidate"); ok {
		t.Error("did not expect structured-video-candidate when VideoObject is already present")
	}
}
