package structured

import (
	"sort"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
)

// maxExamples caps how many page URLs a rolled-up issue carries. Enough to spot-check the
// finding, few enough that a 500-page crawl does not put 500 URLs in one issue's data.
const maxExamples = 5

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

// add records that pageURL declared a node of type typ that was missing the given fields.
func (r *rollup) add(code, typ string, missing []string, pageURL string) {
	if len(missing) == 0 {
		return
	}
	key := rollupKey{code: code, typ: typ}
	e, ok := r.entries[key]
	if !ok {
		e = &rollupEntry{missing: make(map[string]int)}
		r.entries[key] = e
		r.order = append(r.order, key)
	}
	e.pages++
	if len(e.examples) < maxExamples {
		e.examples = append(e.examples, pageURL)
	}
	for _, f := range missing {
		e.missing[f]++
	}
}

// issues renders the accumulated gaps as one issue per code-and-type, attached to the site
// base URL.
func (r *rollup) issues(base string) []analyze.Issue {
	var out []analyze.Issue
	for _, key := range r.order {
		e := r.entries[key]
		out = append(out, analyze.Issue{
			Analyzer: "structured",
			URL:      base,
			Severity: analyze.Info,
			Code:     key.code,
			Message:  rollupMessage(key.code, key.typ),
			Data: map[string]any{
				"type":     key.typ,
				"missing":  e.missing,
				"fields":   sortedFields(e.missing),
				"pages":    e.pages,
				"examples": e.examples,
			},
		})
	}
	return out
}

// rollupMessage phrases the finding for a reader who will not see the code.
func rollupMessage(code, typ string) string {
	switch code {
	case "structured-missing-merchant":
		return typ + " markup is missing Google Merchant listing fields"
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
