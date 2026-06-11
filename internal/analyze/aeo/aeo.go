// Package aeo assesses Answer Engine Optimization: how well a page is structured to be
// surfaced as a direct answer in featured snippets, "People Also Ask", and voice results.
// It is a static, per-page check — it reads the parsed DOM and JSON-LD, never fetching.
//
// The signals are deliberately conservative: a positive note when answer-engine structured
// data is present, and low-noise nudges when a page reads like a Q&A but is not marked up
// or formatted for extraction.
package aeo

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// answerSchemaTypes are the schema.org @types that answer engines consume directly.
var answerSchemaTypes = map[string]bool{
	"FAQPage": true, "QAPage": true, "HowTo": true, "Question": true,
}

const (
	// minQuestionsForFAQ is how many question-style headings a page needs before the absence
	// of FAQ/QA structured data is worth flagging.
	minQuestionsForFAQ = 3
	// maxAnswerWords is the upper bound for an answer that comfortably fits a featured snippet.
	maxAnswerWords = 60
	// minWordsForListFormat gates the snippet-format check so it only fires on substantial
	// long-form content regions.
	minWordsForListFormat = 300
)

// Analyzer scores a page's readiness to be quoted by answer engines.
type Analyzer struct {
	// answerLead enables the opt-in direct-answer-lead check (aeo-no-answer-lead). It is a
	// lower-confidence heuristic, off by default; see Option.
	answerLead bool
}

// Option configures an AEO analyzer.
type Option func(*Analyzer)

// WithAnswerLead enables the opt-in direct-answer-lead check (aeo-no-answer-lead), which is
// off by default.
func WithAnswerLead(on bool) Option { return func(a *Analyzer) { a.answerLead = on } }

// New returns an AEO analyzer configured by opts.
func New(opts ...Option) *Analyzer {
	a := &Analyzer{}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (Analyzer) Name() string { return "aeo" }
func (Analyzer) Description() string {
	return "Answer Engine Optimization: FAQ/HowTo structured data, question headings, concise answers, direct-answer lead, snippet-friendly formatting"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	doc := p.Doc
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "aeo", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data})
	}

	// Answer-engine structured data is a strong positive signal.
	answerSchemas := jsonldAnswerTypes(doc)
	if len(answerSchemas) > 0 {
		add(analyze.Info, "aeo-answer-schema", "Page has answer-engine structured data", map[string]any{"types": answerSchemas})
	}

	// Walk question-style headings, measuring the answer that immediately follows each.
	var questions []string
	doc.Find("h2, h3").Each(func(_ int, s *goquery.Selection) {
		q := collapse(s.Text())
		if !isQuestion(q) {
			return
		}
		questions = append(questions, q)
		answer := collapse(s.NextUntil("h1, h2, h3, h4, h5, h6").Text())
		if words := len(strings.Fields(answer)); words > maxAnswerWords {
			add(analyze.Info, "aeo-answer-too-long", "Answer under a question heading is long for a featured snippet",
				map[string]any{"question": q, "answer_words": words})
		}
	})

	// A page that reads like a Q&A but has no FAQ/QA markup is leaving snippet eligibility
	// on the table.
	if len(questions) >= minQuestionsForFAQ && len(answerSchemas) == 0 {
		add(analyze.Warning, "aeo-faq-candidate", "Page has question-style headings but no FAQPage/QAPage structured data",
			map[string]any{"questions": questions})
	}

	// A question-titled page should answer concisely up front. A missing lead paragraph, or one
	// too long to be a direct answer, buries what snippets and voice results pull from. Opt-in:
	// this heuristic is off unless the analyzer was built WithAnswerLead.
	if h1 := collapse(doc.Find("h1").First().Text()); a.answerLead && isQuestion(h1) {
		content := doc.Find("main, article").First()
		if content.Length() == 0 {
			content = doc.Find("body").First()
		}
		lead := collapse(content.Find("p").First().Text())
		if leadWords := len(strings.Fields(lead)); leadWords == 0 || leadWords > maxAnswerWords {
			add(analyze.Info, "aeo-no-answer-lead", "Question-titled page does not open with a concise direct answer",
				map[string]any{"title": h1, "lead_words": leadWords})
		}
	}

	// Substantial long-form content with no lists or tables is harder for answer engines to
	// extract into a snippet.
	if content := doc.Find("main, article").First(); content.Length() > 0 {
		if words := len(strings.Fields(collapse(content.Text()))); words >= minWordsForListFormat &&
			content.Find("ol, ul, table, dl").Length() == 0 {
			add(analyze.Info, "aeo-no-list-format", "Long-form content has no lists or tables for snippet extraction",
				map[string]any{"words": words})
		}
	}

	return issues
}

// interrogatives are sentence-leading words that mark a heading as a question even without a
// trailing question mark.
var interrogatives = map[string]bool{
	"how": true, "what": true, "why": true, "when": true, "where": true, "who": true,
	"which": true, "can": true, "do": true, "does": true, "is": true, "are": true,
	"should": true, "will": true, "could": true, "would": true, "did": true,
}

// isQuestion reports whether a heading reads as a question: it ends with "?" or opens with an
// interrogative word.
func isQuestion(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.HasSuffix(s, "?") {
		return true
	}
	fields := strings.Fields(strings.ToLower(s))
	return len(fields) > 0 && interrogatives[strings.Trim(fields[0], ",.:;\"'")]
}

// collapse trims and collapses runs of whitespace to single spaces.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// jsonldAnswerTypes returns the deduped answer-engine @types found in the page's JSON-LD.
func jsonldAnswerTypes(doc *goquery.Document) []string {
	seen := map[string]bool{}
	var out []string
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		var v any
		if json.Unmarshal([]byte(raw), &v) != nil {
			return
		}
		for _, t := range collectTypes(v) {
			if answerSchemaTypes[t] && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	})
	return out
}

// collectTypes walks a decoded JSON-LD value collecting every @type, descending into @graph
// arrays and nested objects.
func collectTypes(v any) []string {
	var out []string
	switch t := v.(type) {
	case map[string]any:
		out = append(out, asStrings(t["@type"])...)
		for _, key := range []string{"@graph", "mainEntity"} {
			if g, ok := t[key]; ok {
				out = append(out, collectTypes(g)...)
			}
		}
	case []any:
		for _, item := range t {
			out = append(out, collectTypes(item)...)
		}
	}
	return out
}

func asStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
