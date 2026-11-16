package structured

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// opportunityCode is the code of a ranked structured-data opportunity.
const opportunityCode = "structured-opportunity"

// maxOpportunities caps how many ranked opportunities one crawl emits. High enough that a
// group rarely drops off the list between two crawls of a real site (which gocrawl compare
// would read as resolved), low enough that the list stays a short "fix these first".
const maxOpportunities = 20

// The two gap kinds of an opportunity. Both mean the page earns no rich result for the type
// right now, so they are not weighted against each other; the kind tells the reader whether
// the fix is "add a block" or "add a field".
const (
	gapAbsent     = "absent"     // a candidate check fired: no markup for the type at all
	gapIneligible = "ineligible" // a node of the type exists but misses a required field
)

// typeWeights is an editorial judgement of how visible each type's rich result is in search.
// It is NOT derived from data: it is an opinion, and it needs revisiting as Google changes
// which rich results it shows, the same maintenance the eligibility table carries. Every
// eligibility type and every candidate type must have an entry (a test enforces it), so a new
// type cannot silently score zero.
var typeWeights = map[string]float64{
	"Product": 10, "ProductGroup": 10, // price, stock and rating in the result; shopping surfaces
	"Recipe": 8, "Event": 8, // large dedicated result formats
	"VideoObject": 7,                                                         // video thumbnail and video tab
	"Article":     6, "NewsArticle": 6, "BlogPosting": 6, "LocalBusiness": 6, // article enhancements; local pack
	"BreadcrumbList": 4, "Organization": 4, // small change to the result, but cheap and site-wide
	"FAQPage": 2, "ItemList": 2, "Person": 2, // FAQ results restricted to a few authoritative sites since 2023
	"WebSite": 1, // Sitelinks search box is deprecated
}

// opportunityKey identifies one fix: a URL section, a schema.org type, and a gap kind.
type opportunityKey struct {
	section string
	typ     string
	gap     string
}

// opportunityPage is one counted page of an opportunity, identified by its canonical URL.
type opportunityPage struct {
	canonical string
	key       string // analyze.URLKey(canonical)
	// self is true when the canonical page itself reported the gap, not only a duplicate
	// that points at it.
	self    bool
	missing map[string]bool // ineligible only: union of missing required fields
}

// opportunityGroup accumulates the counted pages of one opportunity.
type opportunityGroup struct {
	pages map[string]*opportunityPage // canonical key -> page
}

// opportunities groups the pages that miss structured data by the fix they share, so the
// report can rank fixes rather than URLs. Like rollup it is created per Analyze call and never
// stored on the Analyzer.
type opportunities struct {
	groups map[opportunityKey]*opportunityGroup
}

// newOpportunities returns an empty accumulator.
func newOpportunities() *opportunities {
	return &opportunities{groups: make(map[opportunityKey]*opportunityGroup)}
}

// add records that page p earns no rich result for typ, through the given gap kind. missing
// lists the absent required fields of an ineligible node and is nil for an absent gap. A page
// open to noindex adds nothing, since schema on it can earn nothing. Pages are counted once per
// canonical URL, and the section comes from the canonical too, so a product reached at
// /products/<h> and /collections/<c>/products/<h> is one page in one group. Membership is a
// set, so the result does not depend on the order pages arrive in. When the canonical page was
// crawled too, issues keeps only the gaps it reported itself (see crawledCanonicals).
func (o *opportunities) add(p *crawler.Page, typ, gap string, missing []string) {
	if !analyze.Indexable(p) {
		return
	}
	canonical := analyze.CanonicalURL(p)
	key := opportunityKey{section: section(canonical), typ: typ, gap: gap}
	g, ok := o.groups[key]
	if !ok {
		g = &opportunityGroup{pages: make(map[string]*opportunityPage)}
		o.groups[key] = g
	}
	ck := analyze.URLKey(canonical)
	pg, seen := g.pages[ck]
	if !seen {
		pg = &opportunityPage{canonical: canonical, key: ck, missing: make(map[string]bool)}
		g.pages[ck] = pg
	}
	if analyze.URLKey(p.FinalURL) == ck {
		pg.self = true
	}
	for _, f := range missing {
		pg.missing[f] = true
	}
}

// crawledCanonicals returns the URL keys of the crawled pages that the structured analyzer
// checked (HTML, 200) and that are their own canonical. Such a page speaks for its URL: a
// duplicate that points at it may lack markup the canonical page has (a variant URL without
// the Product block, say), so a gap reported only by duplicates is dropped when the canonical
// page was checked and did not report it.
func crawledCanonicals(result *crawler.Result) map[string]bool {
	out := make(map[string]bool)
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		if k := analyze.URLKey(p.FinalURL); analyze.URLKey(analyze.CanonicalURL(p)) == k {
			out[k] = true
		}
	}
	return out
}

// section returns the URL section that stands in for a page's template: "/" for the homepage,
// "/*" for a single-segment path, and "/<first-segment>/*" otherwise. Collapsing single-segment
// paths matters: a flat-URL site (/my-post-slug) would otherwise get one group per page.
func section(raw string) string {
	path := raw
	if u, err := url.Parse(raw); err == nil {
		path = u.Path
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return "/"
	}
	first, _, nested := strings.Cut(path, "/")
	if !nested {
		return "/*"
	}
	return "/" + first + "/*"
}

// importance is the per-page priority proxy: 1 + log2(1 + inlinks). The log keeps a homepage
// linked from every page from outweighing a whole template on its own; the 1 + keeps a page
// with no inbound links from counting as zero. Internal links measure how the site treats a
// page, not how many people search for it: this is the one function to replace when Search
// Console data is available.
func importance(inlinks int) float64 {
	return 1 + math.Log2(1+float64(inlinks))
}

// rankedOpportunity is one group with its computed score, ready to sort and render.
type rankedOpportunity struct {
	key     opportunityKey
	score   float64 // rounded to one decimal; ordering uses the rounded value shown
	weight  float64
	pages   []opportunityPage // sorted by importance descending, then canonical URL
	total   int               // inbound links across all pages
	missing []string
}

// issues ranks the accumulated opportunities and renders the top maxOpportunities as issues
// attached to the site base URL. Inbound links to every crawled URL sharing a canonical are
// pooled onto that canonical page.
func (o *opportunities) issues(result *crawler.Result) []analyze.Issue {
	if len(o.groups) == 0 {
		return nil
	}
	pooled := make(map[string]int)
	for p, in := range analyze.Inbound(result) {
		pooled[analyze.URLKey(analyze.CanonicalURL(p))] += in.Count
	}
	checked := crawledCanonicals(result)

	ranked := make([]rankedOpportunity, 0, len(o.groups))
	for key, g := range o.groups {
		r := rankedOpportunity{key: key, weight: typeWeights[key.typ]}
		missing := make(map[string]bool)
		for _, pg := range g.pages {
			if !pg.self && checked[pg.key] {
				continue // the canonical page was checked and has no such gap
			}
			r.pages = append(r.pages, *pg)
			for f := range pg.missing {
				missing[f] = true
			}
		}
		if len(r.pages) == 0 {
			continue
		}
		// Sort before summing so the floating-point total is identical whatever the map order.
		sort.Slice(r.pages, func(i, j int) bool {
			a, b := pooled[r.pages[i].key], pooled[r.pages[j].key]
			if a != b {
				return a > b
			}
			return r.pages[i].canonical < r.pages[j].canonical
		})
		var sum float64
		for _, pg := range r.pages {
			sum += importance(pooled[pg.key])
			r.total += pooled[pg.key]
		}
		r.score = math.Round(r.weight*sum*10) / 10
		if key.gap == gapIneligible {
			r.missing = make([]string, 0, len(missing))
			for f := range missing {
				r.missing = append(r.missing, f)
			}
			sort.Strings(r.missing)
		}
		ranked = append(ranked, r)
	}
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.key.section != b.key.section {
			return a.key.section < b.key.section
		}
		if a.key.typ != b.key.typ {
			return a.key.typ < b.key.typ
		}
		return a.key.gap < b.key.gap
	})
	if len(ranked) > maxOpportunities {
		ranked = ranked[:maxOpportunities]
	}

	base := analyze.SiteBase(result)
	out := make([]analyze.Issue, 0, len(ranked))
	for i, r := range ranked {
		examples := make([]string, 0, maxExamples)
		for _, pg := range r.pages {
			if len(examples) == maxExamples {
				break
			}
			examples = append(examples, pg.canonical)
		}
		data := map[string]any{
			"rank":        i + 1,
			"score":       r.score,
			"type":        r.key.typ,
			"gap":         r.key.gap,
			"section":     r.key.section,
			"pages":       len(r.pages),
			"inlinks":     r.total,
			"type_weight": r.weight,
			"examples":    examples,
			// Many opportunities share code and URL; the key keeps each one distinct when two
			// crawls are compared, and is independent of the rank, which may shift.
			analyze.InstanceKey: r.key.section + "|" + r.key.typ + "|" + r.key.gap,
		}
		if r.key.gap == gapIneligible {
			data["missing"] = r.missing
		}
		out = append(out, analyze.Issue{
			Analyzer: "structured",
			URL:      base,
			// info, not warning: the per-page findings beneath each opportunity are already
			// warnings, and raising these too would double-count them in the report totals.
			Severity: analyze.Info,
			Code:     opportunityCode,
			Message:  opportunityMessage(r.key, len(r.pages)),
			Data:     data,
		})
	}
	return out
}

// opportunityMessage phrases an opportunity for a reader who will not see the code. It leaves
// the rank out, so the message stays the same when the order shifts between crawls.
func opportunityMessage(key opportunityKey, pages int) string {
	where := fmt.Sprintf("%d pages under %s have", pages, key.section)
	switch {
	case key.section == "/":
		where = "The homepage has"
	case pages == 1:
		where = fmt.Sprintf("1 page under %s has", key.section)
	}
	if key.gap == gapIneligible {
		return fmt.Sprintf("%s %s markup missing required fields", where, key.typ)
	}
	return fmt.Sprintf("%s no %s structured data", where, key.typ)
}
