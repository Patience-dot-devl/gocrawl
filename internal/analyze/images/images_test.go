package images_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/images"
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

func codes(issues []analyze.Issue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		out[is.Code] = true
	}
	return out
}

func TestImagesFlagsMissingAltAndDimensions(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img src="/a.png">
		<img src="/b.png" alt="">
		<img src="/c.png" width="10">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(images.New().Analyze(context.Background(), res))

	for _, want := range []string{"img-missing-alt", "img-missing-dimensions"} {
		if !got[want] {
			t.Errorf("expected issue %q, not found", want)
		}
	}
}

func TestImagesEmptyAltIsValid(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img src="/a.png" alt="" width="10" height="10">
		<img src="/b.png" alt="meaningful" width="20" height="20">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(images.New().Analyze(context.Background(), res))

	for _, unwanted := range []string{"img-missing-alt", "img-missing-dimensions"} {
		if got[unwanted] {
			t.Errorf("did not expect issue %q on a clean page", unwanted)
		}
	}
}

func TestImagesNoImagesSkipped(t *testing.T) {
	p := htmlPage(t, `<html><body><p>no images</p></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if len(images.New().Analyze(context.Background(), res)) != 0 {
		t.Error("expected no issues on a page without images")
	}
}
