package urls_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/urls"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

func page(finalURL string) *crawler.Page {
	return &crawler.Page{FinalURL: finalURL, StatusCode: 200}
}

func codes(issues []analyze.Issue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		out[is.Code] = true
	}
	return out
}

func TestURLsFlagsHygieneIssues(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 120)
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"uppercase", "https://example.com/Path/Page", "url-uppercase"},
		{"underscore", "https://example.com/some_path", "url-underscore"},
		{"non-ascii", "https://example.com/café", "url-non-ascii"},
		{"too-long", long, "url-too-long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := &crawler.Result{Pages: []*crawler.Page{page(tc.url)}}
			got := codes(urls.New().Analyze(context.Background(), res))
			if !got[tc.want] {
				t.Errorf("expected issue %q for %q, not found", tc.want, tc.url)
			}
		})
	}
}

func TestURLsCleanURL(t *testing.T) {
	res := &crawler.Result{Pages: []*crawler.Page{page("https://example.com/a-clean/path")}}
	got := codes(urls.New().Analyze(context.Background(), res))
	for _, unwanted := range []string{"url-uppercase", "url-underscore", "url-non-ascii", "url-too-long"} {
		if got[unwanted] {
			t.Errorf("did not expect issue %q on a clean URL", unwanted)
		}
	}
}
