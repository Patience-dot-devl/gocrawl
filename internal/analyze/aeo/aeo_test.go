package aeo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/aeo"
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

func run(t *testing.T, html string) []analyze.Issue {
	t.Helper()
	return aeo.New().Analyze(context.Background(), page(t, html))
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func TestAnswerSchemaDetected(t *testing.T) {
	issues := run(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"FAQPage","mainEntity":[{"@type":"Question","name":"Q?"}]}
	</script></head><body></body></html>`)
	is, ok := find(issues, "aeo-answer-schema")
	if !ok {
		t.Fatal("expected aeo-answer-schema")
	}
	types, _ := is.Data["types"].([]string)
	if len(types) == 0 || types[0] != "FAQPage" {
		t.Errorf("expected FAQPage, got %v", types)
	}
}

func TestFAQCandidateFiresWithoutSchema(t *testing.T) {
	issues := run(t, `<html><body>
		<h2>What is gocrawl?</h2><p>A crawler.</p>
		<h2>How do I install it?</h2><p>go install.</p>
		<h2>Why use it?</h2><p>For audits.</p>
	</body></html>`)
	if _, ok := find(issues, "aeo-faq-candidate"); !ok {
		t.Error("expected aeo-faq-candidate for 3 question headings without schema")
	}
}

func TestFAQCandidateSuppressedBySchema(t *testing.T) {
	issues := run(t, `<html><head><script type="application/ld+json">
		{"@type":"FAQPage"}
	</script></head><body>
		<h2>What is gocrawl?</h2><p>A crawler.</p>
		<h2>How do I install it?</h2><p>go install.</p>
		<h2>Why use it?</h2><p>For audits.</p>
	</body></html>`)
	if _, ok := find(issues, "aeo-faq-candidate"); ok {
		t.Error("aeo-faq-candidate should be suppressed when FAQPage schema is present")
	}
}

func TestFAQCandidateBelowThreshold(t *testing.T) {
	issues := run(t, `<html><body>
		<h2>What is gocrawl?</h2><p>A crawler.</p>
		<h2>Just a heading</h2><p>Not a question.</p>
	</body></html>`)
	if _, ok := find(issues, "aeo-faq-candidate"); ok {
		t.Error("aeo-faq-candidate should not fire with only one question heading")
	}
}

func TestAnswerTooLong(t *testing.T) {
	longAnswer := "word " + strings.Repeat("word ", 70)
	issues := run(t, `<html><body><h2>What is it?</h2><p>`+longAnswer+`</p></body></html>`)
	is, ok := find(issues, "aeo-answer-too-long")
	if !ok {
		t.Fatal("expected aeo-answer-too-long")
	}
	if w, _ := is.Data["answer_words"].(int); w <= 60 {
		t.Errorf("expected >60 answer words, got %v", w)
	}
}

func TestConciseAnswerNotFlagged(t *testing.T) {
	issues := run(t, `<html><body><h2>What is it?</h2><p>A small concise answer.</p></body></html>`)
	if _, ok := find(issues, "aeo-answer-too-long"); ok {
		t.Error("concise answer should not be flagged")
	}
}

func TestNoListFormat(t *testing.T) {
	prose := strings.Repeat("word ", 320)
	issues := run(t, `<html><body><main><p>`+prose+`</p></main></body></html>`)
	if _, ok := find(issues, "aeo-no-list-format"); !ok {
		t.Error("expected aeo-no-list-format for long prose with no lists")
	}
}

func TestListFormatPresent(t *testing.T) {
	prose := strings.Repeat("word ", 320)
	issues := run(t, `<html><body><main><p>`+prose+`</p><ul><li>one</li><li>two</li></ul></main></body></html>`)
	if _, ok := find(issues, "aeo-no-list-format"); ok {
		t.Error("aeo-no-list-format should not fire when a list is present")
	}
}

func TestSkipsNonHTML(t *testing.T) {
	res := &crawler.Result{Pages: []*crawler.Page{{FinalURL: "https://example.com/x.pdf", StatusCode: 200}}}
	if len(aeo.New().Analyze(context.Background(), res)) != 0 {
		t.Error("expected no issues for non-HTML page")
	}
}
