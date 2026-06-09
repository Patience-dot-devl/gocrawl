package utm_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/utm"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// withLinks builds a Result with a single page carrying the given links. The crawler
// populates Page.Links from parsed HTML at crawl time; goquery does not, so link-based
// analyzers are tested by constructing the links directly.
func withLinks(links ...crawler.Link) *crawler.Result {
	return &crawler.Result{Pages: []*crawler.Page{{FinalURL: "https://example.com/", StatusCode: 200, Links: links}}}
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func analyze1(res *crawler.Result) []analyze.Issue {
	return utm.New().Analyze(context.Background(), res)
}

func TestCompleteTaggingNoWarnings(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://ads.example.net/?utm_source=g&utm_medium=cpc&utm_campaign=spring", External: true})
	issues := analyze1(res)
	for _, is := range issues {
		if is.Severity == analyze.Warning {
			t.Errorf("unexpected warning: %s", is.Code)
		}
	}
	sum, ok := find(issues, "utm-summary")
	if !ok {
		t.Fatal("expected utm-summary")
	}
	if sum.Data["tagged_links"] != 1 || sum.Data["external_tagged"] != 1 {
		t.Errorf("summary = %v", sum.Data)
	}
}

func TestPartialTagging(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://ads.example.net/?utm_source=g", External: true})
	is, ok := find(analyze1(res), "utm-partial-tagging")
	if !ok {
		t.Fatal("expected utm-partial-tagging")
	}
	missing, _ := is.Data["missing"].([]string)
	if len(missing) != 2 || missing[0] != "utm_medium" || missing[1] != "utm_campaign" {
		t.Errorf("missing = %v", missing)
	}
}

func TestEmptyValue(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://ads.example.net/?utm_source=g&utm_medium=cpc&utm_campaign=", External: true})
	if _, ok := find(analyze1(res), "utm-empty-value"); !ok {
		t.Error("expected utm-empty-value")
	}
}

func TestDuplicateParam(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://ads.example.net/?utm_term=a&utm_term=b", External: true})
	if _, ok := find(analyze1(res), "utm-duplicate-param"); !ok {
		t.Error("expected utm-duplicate-param")
	}
}

func TestInconsistentCasing(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://ads.example.net/?UTM_Source=g&utm_medium=cpc&utm_campaign=x", External: true})
	if _, ok := find(analyze1(res), "utm-inconsistent-casing"); !ok {
		t.Error("expected utm-inconsistent-casing")
	}
}

func TestInternalTagged(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://example.com/page?utm_source=g&utm_medium=cpc&utm_campaign=x", External: false})
	issues := analyze1(res)
	if _, ok := find(issues, "utm-internal-tagged"); !ok {
		t.Error("expected utm-internal-tagged")
	}
	sum, _ := find(issues, "utm-summary")
	if sum.Data["internal_tagged"] != 1 {
		t.Errorf("internal_tagged = %v", sum.Data["internal_tagged"])
	}
}

func TestUntaggedOnly(t *testing.T) {
	res := withLinks(crawler.Link{URL: "https://example.com/about", External: false})
	issues := analyze1(res)
	if _, ok := find(issues, "utm-partial-tagging"); ok {
		t.Error("did not expect any tagging issue for an untagged link")
	}
	sum, ok := find(issues, "utm-summary")
	if !ok {
		t.Fatal("expected utm-summary")
	}
	if sum.Data["tagged_links"] != 0 {
		t.Errorf("tagged_links = %v, want 0", sum.Data["tagged_links"])
	}
}
