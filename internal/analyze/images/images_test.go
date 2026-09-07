package images_test

import (
	"context"
	"slices"
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

func find(t *testing.T, issues []analyze.Issue, code string) analyze.Issue {
	t.Helper()
	for _, is := range issues {
		if is.Code == code {
			return is
		}
	}
	t.Fatalf("expected issue %q, got %v", code, codes(issues))
	return analyze.Issue{}
}

// srcs unwraps the []string stored in an issue's Data map.
func srcs(v any) []string {
	out, _ := v.([]string)
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

func TestImagesEmptyAltIsReportedButNotAProblem(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img src="/decor.png" alt="" width="10" height="10">
		<img src="/hero-product.png" alt="meaningful" width="20" height="20">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := images.New().Analyze(context.Background(), res)
	got := codes(issues)

	for _, unwanted := range []string{"img-missing-alt", "img-missing-dimensions", "img-duplicate-alt", "img-alt-is-filename"} {
		if got[unwanted] {
			t.Errorf("did not expect issue %q on a clean page", unwanted)
		}
	}
	is := find(t, issues, "img-empty-alt")
	if is.Severity != analyze.Info {
		t.Errorf("img-empty-alt severity = %q, want info", is.Severity)
	}
	if got, want := is.Data["count"], 1; got != want {
		t.Errorf("count = %v, want %v", got, want)
	}
	if got, want := is.Data["images_total"], 2; got != want {
		t.Errorf("images_total = %v, want %v", got, want)
	}
	if got, want := srcs(is.Data["sources"]), []string{"https://example.com/decor.png"}; !slices.Equal(got, want) {
		t.Errorf("sources = %v, want %v", got, want)
	}
}

func TestImagesEmptyAltDistinctFromMissingAlt(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img src="/a.png" width="1" height="1">
		<img src="/b.png" alt="" width="1" height="1">
		<img src="/c.png" alt="   " width="1" height="1">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := images.New().Analyze(context.Background(), res)

	if got, want := find(t, issues, "img-missing-alt").Data["count"], 1; got != want {
		t.Errorf("missing-alt count = %v, want %v", got, want)
	}
	// Whitespace-only alt is the same thing as alt="" once the browser trims it.
	if got, want := find(t, issues, "img-empty-alt").Data["count"], 2; got != want {
		t.Errorf("empty-alt count = %v, want %v", got, want)
	}
}

func TestImagesDuplicateAltWithinPage(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img src="/cdn/shop/files/hs1-front.png" alt="Defibrillator" width="1" height="1">
		<img src="/cdn/shop/files/hs1-side.png" alt="defibrillator" width="1" height="1">
		<img src="/cdn/shop/files/hs1-case.png" alt="Defibrillator" width="1" height="1">
		<img src="/cdn/shop/files/pads-set.png" alt="Electrode pads" width="1" height="1">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := images.New().Analyze(context.Background(), res)

	is := find(t, issues, "img-duplicate-alt")
	if is.Severity != analyze.Warning {
		t.Errorf("severity = %q, want warning", is.Severity)
	}
	if got, want := is.Data["count"], 3; got != want {
		t.Errorf("count = %v, want %v (three images share one alt)", got, want)
	}
	dups, ok := is.Data["duplicates"].([]map[string]any)
	if !ok || len(dups) != 1 {
		t.Fatalf("duplicates = %#v, want one group", is.Data["duplicates"])
	}
	if got, want := dups[0]["alt"], "Defibrillator"; got != want {
		t.Errorf("duplicate alt = %v, want %v", got, want)
	}
}

func TestImagesAltIsFilename(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img src="/files/kantoor.jpg" alt="Kantoor" width="1" height="1">
		<img src="/files/team-photo.jpg" alt="team_photo.jpg" width="1" height="1">
		<img src="/files/warehouse.jpg" alt="Our warehouse in Rotterdam" width="1" height="1">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := images.New().Analyze(context.Background(), res)

	is := find(t, issues, "img-alt-is-filename")
	if got, want := srcs(is.Data["sources"]), []string{
		"https://example.com/files/kantoor.jpg",
		"https://example.com/files/team-photo.jpg",
	}; !slices.Equal(got, want) {
		t.Errorf("sources = %v, want %v", got, want)
	}
}

func TestImagesNonDescriptiveFilename(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img src="/files/7.jpg" alt="A defibrillator on a wall" width="1" height="1">
		<img src="/files/hs1.png" alt="HeartStart HS1" width="1" height="1">
		<img src="/files/IMG_1234.jpg" alt="Training session" width="1" height="1">
		<img src="/files/aed-wall-cabinet.jpg" alt="Wall cabinet" width="1" height="1">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := images.New().Analyze(context.Background(), res)

	is := find(t, issues, "img-nondescriptive-filename")
	if is.Severity != analyze.Info {
		t.Errorf("severity = %q, want info", is.Severity)
	}
	if got, want := is.Data["count"], 3; got != want {
		t.Errorf("count = %v, want %v", got, want)
	}
	for _, bad := range []string{"aed-wall-cabinet"} {
		for _, s := range srcs(is.Data["sources"]) {
			if strings.Contains(s, bad) {
				t.Errorf("descriptive filename %q was flagged", s)
			}
		}
	}
}

func TestImagesLazyLoadedSrcAndSrcset(t *testing.T) {
	p := htmlPage(t, `<html><body>
		<img data-src="/cdn/shop/files/kantoor.jpg?v=17&width=1000" alt="" width="1" height="1">
		<img srcset="/cdn/shop/files/9.png?v=2 1x, /cdn/shop/files/9@2x.png 2x" alt="" width="1" height="1">
	</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := images.New().Analyze(context.Background(), res)

	if got, want := srcs(find(t, issues, "img-empty-alt").Data["sources"]), []string{
		"https://example.com/cdn/shop/files/kantoor.jpg?v=17&width=1000",
		"https://example.com/cdn/shop/files/9.png?v=2",
	}; !slices.Equal(got, want) {
		t.Errorf("sources = %v, want %v", got, want)
	}
	// The query string must not leak into the filename check.
	if got, want := find(t, issues, "img-nondescriptive-filename").Data["count"], 1; got != want {
		t.Errorf("non-descriptive count = %v, want %v", got, want)
	}
}

func TestImagesNoImagesSkipped(t *testing.T) {
	p := htmlPage(t, `<html><body><p>no images</p></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if len(images.New().Analyze(context.Background(), res)) != 0 {
		t.Error("expected no issues on a page without images")
	}
}
