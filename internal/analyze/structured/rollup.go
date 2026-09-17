package structured

import (
	"sort"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
)

// maxExamples caps how many page URLs a rolled-up issue carries. Enough to spot-check the
// finding, few enough that a 500-page crawl does not put 500 URLs in one issue's data.
const maxExamples = 5

// breadcrumbCandidateCode is the one rollup code that carries a datum beyond the shared shape:
// links, the largest breadcrumb link count seen on a counted page.
const breadcrumbCandidateCode = "structured-breadcrumb-candidate"

// rollupKey identifies one aggregated finding: an issue code and the schema.org type it was
// raised against.
type rollupKey struct {
	code string
	typ  string
}

// rollupEntry accumulates one key's occurrences across the crawl.
type rollupEntry struct {
	missing  map[string]int // field (or any-of group) -> number of pages missing it
	pages    int
	examples []string
	links    int // largest breadcrumb link count seen; rendered only for breadcrumbCandidateCode
	// owner maps a canonical URL key (analyze.URLKey) to the FinalURL of the page that claimed
	// it, and counted to the fields already tallied for it. Together they make every count
	// here a count of distinct canonical pages: a product reached at /products/<h> and again
	// at /collections/<c>/products/<h> is one page, however many nodes of the type it carries.
	owner   map[string]string
	counted map[string]map[string]bool
}

// rollup aggregates the field gaps that recur on every page of a template. Recommended and
// merchant fields are absent site-wide far more often than they are absent on one page — a
// theme either emits aggregateRating or it does not — so reporting them per page would add
// one issue per product for no extra information. The accumulator is created per Analyze
// call and never stored on the Analyzer, keeping the analyzer safe for sequential reuse.
type rollup struct {
	entries map[rollupKey]*rollupEntry
	order   []rollupKey
}

// newRollup returns an empty accumulator.
func newRollup() *rollup {
	return &rollup{entries: make(map[rollupKey]*rollupEntry)}
}

// add records that the page at pageURL, whose canonical URL is canonical, declared a node of
// type typ missing the given fields. Counts are per canonical URL: a different page with the
// same canonical adds nothing, whichever of the two was crawled first, and the example is the
// canonical URL so it does not depend on crawl order either. A second node of the type on the
// same page does not count the page again; it only adds fields that page was not yet counted
// as missing. It returns the entry the page was counted in, or nil when the page added nothing
// (no missing fields, or its canonical URL already belongs to a different page).
func (r *rollup) add(code, typ string, missing []string, pageURL, canonical string) *rollupEntry {
	if len(missing) == 0 {
		return nil
	}
	key := rollupKey{code: code, typ: typ}
	e, ok := r.entries[key]
	if !ok {
		e = &rollupEntry{
			missing: make(map[string]int),
			owner:   make(map[string]string),
			counted: make(map[string]map[string]bool),
		}
		r.entries[key] = e
		r.order = append(r.order, key)
	}
	ck := analyze.URLKey(canonical)
	owner, seen := e.owner[ck]
	if seen && owner != pageURL {
		return nil
	}
	if !seen {
		e.owner[ck] = pageURL
		e.counted[ck] = make(map[string]bool)
		e.pages++
		if len(e.examples) < maxExamples {
			e.examples = append(e.examples, canonical)
		}
	}
	for _, f := range missing {
		if !e.counted[ck][f] {
			e.counted[ck][f] = true
			e.missing[f]++
		}
	}
	return e
}

// addBreadcrumb records a page that renders breadcrumb navigation with the given number of
// links but carries no BreadcrumbList. The breadcrumb trail is template chrome, so it rolls up
// like a field gap: one finding per crawl, with missing keyed by the absent type. links keeps
// the maximum across counted pages, which does not depend on the order pages arrive in.
func (r *rollup) addBreadcrumb(pageURL, canonical string, links int) {
	if e := r.add(breadcrumbCandidateCode, "BreadcrumbList", []string{"BreadcrumbList"}, pageURL, canonical); e != nil && links > e.links {
		e.links = links
	}
}

// issues renders the accumulated gaps as one issue per code-and-type, attached to the site
// base URL.
func (r *rollup) issues(base string) []analyze.Issue {
	var out []analyze.Issue
	for _, key := range r.order {
		e := r.entries[key]
		data := map[string]any{
			"type":     key.typ,
			"missing":  e.missing,
			"fields":   sortedFields(e.missing),
			"pages":    e.pages,
			"examples": e.examples,
			// One issue per type shares code and URL with its siblings; the type keeps
			// each one distinct when two crawls are compared.
			analyze.InstanceKey: key.typ,
		}
		if key.code == breadcrumbCandidateCode {
			data["links"] = e.links
		}
		out = append(out, analyze.Issue{
			Analyzer: "structured",
			URL:      base,
			Severity: rollupSeverity(key.code),
			Code:     key.code,
			Message:  rollupMessage(key.code, key.typ),
			Data:     data,
		})
	}
	return out
}

// rollupSeverity distinguishes the two rolled-up codes: the merchant tier is the commercial
// point of this analyzer (Google Shopping / free-listing eligibility), so it warrants warning,
// while the recommended tier is genuinely optional polish and stays info. Incomplete variants
// are warning for the same reason: Google requires those fields on each inline variant. An
// identifier found only on Offer is warning because it is the merchant identifier gap, reworded
// so the store moves data it has rather than looks for data it lacks. The breadcrumb candidate
// keeps the warning it had as a per-page finding: the rich result is one JSON-LD block away.
func rollupSeverity(code string) analyze.Severity {
	switch code {
	case "structured-missing-merchant", "structured-variant-incomplete", "structured-identifier-on-offer", breadcrumbCandidateCode:
		return analyze.Warning
	}
	return analyze.Info
}

// rollupMessage phrases the finding for a reader who will not see the code.
func rollupMessage(code, typ string) string {
	switch code {
	case "structured-missing-merchant":
		return typ + " markup is missing Google Merchant listing fields"
	case "structured-variant-incomplete":
		return typ + " variants are missing fields Google requires on each variant"
	case "structured-identifier-on-offer":
		return typ + " markup declares GTIN/MPN on Offer, where Google's merchant listings do not document reading it"
	case breadcrumbCandidateCode:
		return "Pages render breadcrumb navigation but carry no BreadcrumbList structured data"
	default:
		return typ + " markup is missing recommended schema.org fields"
	}
}

// sortedFields returns the missing field names in a stable order, so a report diffed between
// two crawls does not churn on Go's randomized map iteration.
func sortedFields(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for f := range m {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
