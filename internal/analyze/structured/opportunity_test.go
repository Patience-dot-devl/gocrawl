package structured_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// site is a small crawl under construction: pages by URL, plus the internal links between them.
type site struct {
	t     *testing.T
	pages []*crawler.Page
	byURL map[string]*crawler.Page
}

func newSite(t *testing.T) *site {
	return &site{t: t, byURL: make(map[string]*crawler.Page)}
}

// add adds a 200 HTML page at u with the given HTML and returns it.
func (s *site) add(u, html string) *crawler.Page {
	s.t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		s.t.Fatalf("parse %s: %v", u, err)
	}
	p := &crawler.Page{RequestedURL: u, FinalURL: u, StatusCode: 200, ContentType: "text/html", Doc: doc, Header: http.Header{}}
	s.pages = append(s.pages, p)
	s.byURL[u] = p
	return p
}

// fixture adds a page at u whose HTML is testdata/opportunity/<name>.
func (s *site) fixture(u, name string) *crawler.Page {
	s.t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "opportunity", name))
	if err != nil {
		s.t.Fatal(err)
	}
	return s.add(u, string(b))
}

// link adds n internal links from the page at from to the page at to.
func (s *site) link(from, to string, n int) {
	p := s.byURL[from]
	for i := 0; i < n; i++ {
		p.Links = append(p.Links, crawler.Link{URL: to})
	}
}

// result returns the crawl with its pages in the given order (nil: insertion order).
func (s *site) result(order []*crawler.Page) *crawler.Result {
	if order == nil {
		order = s.pages
	}
	res := &crawler.Result{Seed: "https://shop.test/", Pages: append([]*crawler.Page(nil), order...)}
	res.Reindex()
	return res
}

func opportunitiesOf(res *crawler.Result) []analyze.Issue {
	return findAll(structured.New().Analyze(context.Background(), res), "structured-opportunity")
}

const productHTML = `<html><body><h1>Tee</h1><form><span>$19.99</span><button>Add to cart</button></form></body></html>`

// articleHTML passes the article candidate check: a long <article> with a publish date.
var articleHTML = `<html><body><article><time datetime="2026-01-01">Jan 1</time><p>` +
	strings.Repeat("word ", 200) + `</p></article></body></html>`

const hub = "https://shop.test/collections/all"

func TestOpportunityTemplateOutranksOneOffPage(t *testing.T) {
	s := newSite(t)
	s.add(hub, `<html><body></body></html>`)
	for i := 0; i < 5; i++ {
		s.add(fmt.Sprintf("https://shop.test/products/p%d", i), productHTML)
	}
	s.add("https://shop.test/blogs/news/launch", articleHTML)
	s.link(hub, "https://shop.test/blogs/news/launch", 3)

	got := opportunitiesOf(s.result(nil))
	if len(got) != 2 {
		t.Fatalf("expected 2 opportunities, got %d: %+v", len(got), got)
	}
	first, second := got[0].Data, got[1].Data
	if first["type"] != "Product" || first["section"] != "/products/*" || first["gap"] != "absent" || first["pages"] != 5 {
		t.Errorf("first opportunity = %v, want the 5-page Product template", first)
	}
	if second["type"] != "Article" || second["section"] != "/blogs/*" {
		t.Errorf("second opportunity = %v, want the blog Article", second)
	}
	// Product: 10 x 5 pages x importance 1. Article: 6 x (1 + log2(4)) = 18.
	if first["score"] != 50.0 || second["score"] != 18.0 {
		t.Errorf("scores = %v, %v; want 50, 18", first["score"], second["score"])
	}
	if first["rank"] != 1 || second["rank"] != 2 {
		t.Errorf("ranks = %v, %v", first["rank"], second["rank"])
	}
	if second["inlinks"] != 3 || second["type_weight"] != 6.0 {
		t.Errorf("article inlinks/type_weight = %v/%v", second["inlinks"], second["type_weight"])
	}
	is := got[0]
	if is.URL != "https://shop.test" || is.Severity != analyze.Info || is.Analyzer != "structured" {
		t.Errorf("unexpected envelope: %+v", is)
	}
	if is.Message != "5 pages under /products/* have no Product structured data" {
		t.Errorf("message = %q", is.Message)
	}
	if is.Data[analyze.InstanceKey] != "/products/*|Product|absent" {
		t.Errorf("instance = %v", is.Data[analyze.InstanceKey])
	}
	if _, ok := is.Data["missing"]; ok {
		t.Error("absent opportunity must not carry missing")
	}
}

func TestOpportunityTieBreakAndOrderIndependence(t *testing.T) {
	s := newSite(t)
	// Two equal-scoring Product groups: the section decides, ascending.
	s.add("https://shop.test/zeta/a", productHTML)
	s.add("https://shop.test/alpha/a", productHTML)
	s.add("https://shop.test/products/b", productHTML)
	s.add("https://shop.test/products/a", productHTML)
	s.add(hub, `<html><body></body></html>`)
	s.link(hub, "https://shop.test/products/b", 7)
	s.link(hub, "https://shop.test/products/a", 1)

	fwd := s.result(nil)
	rev := make([]*crawler.Page, len(s.pages))
	for i, p := range s.pages {
		rev[len(s.pages)-1-i] = p
	}
	a, b := opportunitiesOf(fwd), opportunitiesOf(s.result(rev))
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("ranking depends on page order:\n%+v\n%+v", a, b)
	}
	var sections []string
	for _, is := range a {
		sections = append(sections, is.Data["section"].(string))
	}
	if strings.Join(sections, " ") != "/products/* /alpha/* /zeta/*" {
		t.Errorf("order = %v, want /products/* then the tie broken by section", sections)
	}
	if ex := a[0].Data["examples"].([]string); strings.Join(ex, " ") != "https://shop.test/products/b https://shop.test/products/a" {
		t.Errorf("examples = %v, want most-linked first", ex)
	}
}

func TestOpportunityExcludesNoindexPages(t *testing.T) {
	s := newSite(t)
	s.add("https://shop.test/products/a", productHTML)
	s.add("https://shop.test/products/meta", `<html><head><meta name="robots" content="noindex"></head><body><form><span>$1.00</span><button>Add to cart</button></form></body></html>`)
	s.add("https://shop.test/products/header", productHTML).Header.Set("X-Robots-Tag", "noindex")
	got := opportunitiesOf(s.result(nil))
	if len(got) != 1 || got[0].Data["pages"] != 1 {
		t.Fatalf("expected one 1-page opportunity, got %+v", got)
	}
}

func TestOpportunityCountsCanonicalOnceAndPoolsInlinks(t *testing.T) {
	s := newSite(t)
	canon := `<link rel="canonical" href="https://shop.test/products/tee">`
	html := `<html><head>` + canon + `</head><body><form><span>$19.99</span><button>Add to cart</button></form></body></html>`
	s.add("https://shop.test/products/tee", html)
	s.add("https://shop.test/collections/all/products/tee", html)
	s.add(hub, `<html><body></body></html>`)
	s.link(hub, "https://shop.test/products/tee", 1)
	s.link(hub, "https://shop.test/collections/all/products/tee", 2)

	got := opportunitiesOf(s.result(nil))
	if len(got) != 1 {
		t.Fatalf("expected one opportunity, got %+v", got)
	}
	d := got[0].Data
	if d["pages"] != 1 || d["inlinks"] != 3 || d["section"] != "/products/*" {
		t.Errorf("data = %v, want 1 page with 3 pooled inlinks under /products/*", d)
	}
	if ex := d["examples"].([]string); len(ex) != 1 || ex[0] != "https://shop.test/products/tee" {
		t.Errorf("examples = %v, want the canonical", ex)
	}
}

func TestOpportunitySections(t *testing.T) {
	s := newSite(t)
	s.add("https://shop.test/", productHTML)
	s.add("https://shop.test/my-first-post", articleHTML)
	s.add("https://shop.test/my-second-post/", articleHTML)
	s.add("https://shop.test/blogs/news/a/b", articleHTML)
	got := opportunitiesOf(s.result(nil))
	bySection := map[string]analyze.Issue{}
	for _, is := range got {
		bySection[is.Data["section"].(string)] = is
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 groups (/, /*, /blogs/*), got %d: %+v", len(got), got)
	}
	if bySection["/*"].Data["pages"] != 2 {
		t.Errorf("flat URLs must share one /* group, got %+v", bySection["/*"])
	}
	if bySection["/"].Message != "The homepage has no Product structured data" {
		t.Errorf("homepage message = %q", bySection["/"].Message)
	}
	if _, ok := bySection["/blogs/*"]; !ok {
		t.Error("expected a /blogs/* group")
	}
}

func TestOpportunityGapSources(t *testing.T) {
	s := newSite(t)
	s.add("https://shop.test/products/a", productHTML)
	s.add("https://shop.test/blogs/a", articleHTML)
	s.add("https://shop.test/videos/a", `<html><body><iframe src="https://player.vimeo.com/video/1"></iframe></body></html>`)
	s.add("https://shop.test/pages/a", `<html><body><nav class="breadcrumb"><a href="/">Home</a><a href="/pages">Pages</a></nav></body></html>`)
	s.add("https://shop.test/news/a", `<html><head><script type="application/ld+json">{"@context":"https://schema.org","@type":"VideoObject","name":"Clip"}</script></head><body></body></html>`)
	s.add("https://shop.test/news/b", `<html><head><script type="application/ld+json">{"@context":"https://schema.org","@type":"VideoObject","thumbnailUrl":"https://shop.test/t.jpg","name":"Clip","description":"d"}</script></head><body></body></html>`)

	want := map[string]string{
		"/products/*|Product|absent":     "",
		"/blogs/*|Article|absent":        "",
		"/videos/*|VideoObject|absent":   "",
		"/pages/*|BreadcrumbList|absent": "",
		"/news/*|VideoObject|ineligible": "description thumbnailUrl uploadDate",
	}
	got := opportunitiesOf(s.result(nil))
	if len(got) != len(want) {
		t.Fatalf("expected %d opportunities, got %d: %+v", len(want), len(got), got)
	}
	for _, is := range got {
		inst := is.Data[analyze.InstanceKey].(string)
		missing, ok := want[inst]
		if !ok {
			t.Errorf("unexpected opportunity %s", inst)
			continue
		}
		if missing == "" {
			if _, has := is.Data["missing"]; has {
				t.Errorf("%s: absent gap must not carry missing", inst)
			}
			continue
		}
		if m := strings.Join(is.Data["missing"].([]string), " "); m != missing {
			t.Errorf("%s: missing = %q, want the sorted union %q", inst, m, missing)
		}
		if is.Message != "2 pages under /news/* have VideoObject markup missing required fields" {
			t.Errorf("%s: message = %q", inst, is.Message)
		}
	}
}

func TestOpportunityCapsAtTwenty(t *testing.T) {
	s := newSite(t)
	for i := 0; i < 25; i++ {
		s.add(fmt.Sprintf("https://shop.test/s%02d/p", i), productHTML)
	}
	got := opportunitiesOf(s.result(nil))
	if len(got) != 20 {
		t.Fatalf("expected the top 20, got %d", len(got))
	}
	if got[19].Data["rank"] != 20 || got[19].Data["section"] != "/s19/*" {
		t.Errorf("last kept = %v", got[19].Data)
	}
}

// TestOpportunityFixtures runs the analyzer over the HTML fixtures under testdata/opportunity
// and checks the full emitted ranking.
func TestOpportunityFixtures(t *testing.T) {
	s := newSite(t)
	s.add("https://shop.test/", `<html><body></body></html>`)
	s.fixture("https://shop.test/products/tee", "product.html")
	s.fixture("https://shop.test/products/sock", "product.html")
	s.fixture("https://shop.test/products/hidden", "product-noindex.html")
	s.fixture("https://shop.test/blogs/care/wash", "article.html")
	s.fixture("https://shop.test/pages/lookbook", "video.html")
	s.fixture("https://shop.test/blogs/news/moved", "blogposting-no-headline.html")
	s.link("https://shop.test/", "https://shop.test/products/tee", 1)
	s.link("https://shop.test/", "https://shop.test/blogs/care/wash", 1)

	type row struct {
		Rank     int
		Instance string
		Score    float64
		Pages    int
		Examples string
	}
	var got []row
	for _, is := range opportunitiesOf(s.result(nil)) {
		got = append(got, row{
			Rank: is.Data["rank"].(int), Instance: is.Data[analyze.InstanceKey].(string),
			Score: is.Data["score"].(float64), Pages: is.Data["pages"].(int),
			Examples: strings.Join(is.Data["examples"].([]string), " "),
		})
	}
	want := []row{
		// 10 x ((1 + log2 2) + 1) = 30
		{1, "/products/*|Product|absent", 30, 2, "https://shop.test/products/tee https://shop.test/products/sock"},
		// 6 x (1 + log2 2) = 12
		{2, "/blogs/*|Article|absent", 12, 1, "https://shop.test/blogs/care/wash"},
		// 7 x 1 = 7
		{3, "/pages/*|VideoObject|absent", 7, 1, "https://shop.test/pages/lookbook"},
		// 6 x 1 = 6; the noindex product page is not counted anywhere
		{4, "/blogs/*|BlogPosting|ineligible", 6, 1, "https://shop.test/blogs/news/moved"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ranking:\n got %+v\nwant %+v", got, want)
	}
}

// TestOpportunityIgnoresDuplicateWhenCanonicalHasMarkup: a variant URL without the Product
// block that points its canonical at a crawled page with Product markup is not an opportunity
// for that canonical page. When the canonical page was not crawled, the duplicate still counts.
func TestOpportunityIgnoresDuplicateWhenCanonicalHasMarkup(t *testing.T) {
	canon := `<link rel="canonical" href="https://shop.test/products/tee">`
	withProduct := `<html><head>` + canon + `<script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","name":"Tee","image":"https://shop.test/tee.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"EUR"}}</script></head>
		<body><form><span>$19.99</span><button>Add to cart</button></form></body></html>`
	variant := `<html><head>` + canon + `</head><body><form><span>$19.99</span><button>Add to cart</button></form></body></html>`

	s := newSite(t)
	s.add("https://shop.test/products/tee", withProduct)
	s.add("https://shop.test/products/tee-red", variant)
	for _, o := range opportunitiesOf(s.result(nil)) {
		if o.Data["type"] == "Product" && o.Data["gap"] == "absent" {
			t.Errorf("canonical page has Product markup, but got opportunity %v", o.Data)
		}
	}

	s = newSite(t)
	s.add("https://shop.test/products/tee-red", variant)
	got := opportunitiesOf(s.result(nil))
	if len(got) != 1 || got[0].Data["type"] != "Product" || got[0].Data["pages"] != 1 {
		t.Errorf("canonical not crawled: want the duplicate's Product opportunity, got %+v", got)
	}
}
