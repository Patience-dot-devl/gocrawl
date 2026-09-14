package structured

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// singletonTypes describe the page itself rather than something on it, so a page should carry
// exactly one of each. Two, in two different script blocks, almost always means two sources —
// a theme and an SEO app, or two SEO apps — each describing the page its own way. Search
// engines pick one, and which one is not up to the site.
var singletonTypes = []string{"Product", "ProductGroup", "Organization", "BreadcrumbList", "WebSite"}

// conflictFields are the properties worth comparing between duplicate nodes. A disagreement
// here is not a stylistic difference; it is two answers to a question with one right answer.
var conflictFields = []string{"name", "sku", "offers.price", "offers.priceCurrency", "offers.availability"}

// integrityIssues reports structured data that contradicts itself: the same page-level type
// declared by two sources, duplicate declarations that disagree on a key value, and @id
// references pointing at nodes that are not on the page.
func integrityIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	var issues []analyze.Issue
	issues = append(issues, duplicateIssues(p, g)...)
	issues = append(issues, unresolvedIDIssues(p, g)...)
	issues = append(issues, valueIssues(p, g)...)
	return issues
}

// duplicateIssues flags singleton types declared in more than one block, and reports any key
// field on which those declarations disagree.
func duplicateIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	var issues []analyze.Issue
	for _, ty := range singletonTypes {
		nodes := topLevelOfType(g, ty)
		blocks := distinctBlocks(nodes)
		if len(blocks) < 2 {
			continue
		}
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
			Code:    "structured-duplicate-type",
			Message: "Page declares " + ty + " structured data in more than one JSON-LD block",
			Data:    map[string]any{"type": ty, "blocks": len(blocks)},
		})
		for _, field := range conflictFields {
			if values := disagreement(g, nodes, field); len(values) > 1 {
				issues = append(issues, analyze.Issue{
					Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Error,
					Code:    "structured-conflicting-value",
					Message: "Duplicate " + ty + " blocks disagree on " + field,
					Data:    map[string]any{"type": ty, "field": field, "values": values},
				})
			}
		}
	}
	return issues
}

// topLevelOfType returns the nodes of a type that are not thin copies inside a list property,
// so a collection page's product tiles do not read as duplicates of each other.
func topLevelOfType(g schemaorg.Graph, ty string) []schemaorg.Node {
	var out []schemaorg.Node
	for _, n := range g.OfType(ty) {
		if !exemptFromEligibility(n.Path) {
			out = append(out, n)
		}
	}
	return out
}

// distinctBlocks returns the set of script blocks the nodes came from.
func distinctBlocks(nodes []schemaorg.Node) map[int]bool {
	blocks := make(map[int]bool)
	for _, n := range nodes {
		blocks[n.Block] = true
	}
	return blocks
}

// disagreement returns the distinct non-empty values the nodes give for a field, sorted for
// stable output. One value means they agree; more than one means they do not.
func disagreement(g schemaorg.Graph, nodes []schemaorg.Node, field string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, n := range nodes {
		v := g.Str(n, field)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// idRef pairs a property name with the @id it references, so unresolvedIDIssues can sort
// them into a deterministic order before de-duplicating and emitting issues. n.Props is a
// map, and Go randomizes map iteration order per run; ranging it directly would make both the
// order of emitted issues and — via the seen de-dup — which property "wins" for a repeated id
// a coin flip.
type idRef struct {
	key string
	id  string
}

// unresolvedIDIssues flags {"@id": ...} stubs whose target is not declared anywhere on the
// page. A dangling reference silently drops whatever the property was meant to convey — a
// publisher, a brand, a parent product.
func unresolvedIDIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	seen := make(map[string]bool)
	var issues []analyze.Issue
	for _, n := range g.Nodes {
		var refs []idRef
		for key, v := range n.Props {
			if strings.HasPrefix(key, "@") {
				continue
			}
			for _, id := range referencedIDs(v) {
				refs = append(refs, idRef{key: key, id: id})
			}
		}
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].key != refs[j].key {
				return refs[i].key < refs[j].key
			}
			return refs[i].id < refs[j].id
		})
		for _, ref := range refs {
			if seen[ref.id] {
				continue
			}
			if _, ok := g.Resolve(ref.id); ok {
				continue
			}
			seen[ref.id] = true
			issues = append(issues, analyze.Issue{
				Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
				Code:    "structured-unresolved-id",
				Message: "A JSON-LD @id reference points at a node that is not on the page",
				Data:    map[string]any{"id": ref.id, "property": ref.key, "type": firstType(n)},
			})
		}
	}
	return issues
}

// referencedIDs returns the @id values of any bare reference stubs in v. A stub is an object
// whose only key is @id — an object carrying an @id alongside real properties is a
// declaration, not a reference.
func referencedIDs(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, item := range t {
			out = append(out, referencedIDs(item)...)
		}
		return out
	case map[string]any:
		if len(t) != 1 {
			return nil
		}
		if id, ok := t["@id"].(string); ok && id != "" {
			return []string{id}
		}
	}
	return nil
}

// firstType returns a node's primary @type for use in a finding's data.
func firstType(n schemaorg.Node) string {
	if len(n.Types) == 0 {
		return ""
	}
	return n.Types[0]
}

// urlProperties hold values that must resolve on their own. A search engine reads structured
// data out of the page's context, so a relative path in markup resolves against nothing.
var urlProperties = []string{"url", "image", "logo", "thumbnailUrl", "contentUrl", "embedUrl", "sameAs"}

// dateProperties must carry ISO 8601. A locale-formatted date is silently unparseable, which
// costs the page whatever the date was signalling — article freshness, event timing, an offer
// expiry.
var dateProperties = []string{
	"datePublished", "dateModified", "uploadDate",
	"startDate", "endDate", "validFrom", "priceValidUntil",
	"offers.priceValidUntil", "offers.validFrom",
}

// priceProperties must carry a bare decimal. Google's documentation is explicit that a price
// may not include currency symbols, thousands separators, or a range.
var priceProperties = []string{"price", "offers.price", "lowPrice", "highPrice", "offers.lowPrice", "offers.highPrice"}

// isoDateLayouts are the ISO 8601 shapes schema.org accepts, most specific first.
var isoDateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
	"2006-01",
	"2006",
}

// bareDecimalRe matches a price a search engine can parse: digits, optionally one decimal
// point, nothing else.
var bareDecimalRe = regexp.MustCompile(`^\d+(\.\d+)?$`)

// digitsRe pulls the numeric part out of a rendered on-page price such as "$1,299.00".
var digitsRe = regexp.MustCompile(`\d[\d,.]*\d|\d`)

// valueIssues checks the formats of individual property values, and whether the price in the
// markup agrees with the price the page shows a visitor.
func valueIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	var issues []analyze.Issue
	// A nested node is reachable both on its own and through its parent's dotted path, so a
	// bad price on an Offer inside a Product would otherwise be reported twice — once as
	// "offers.price" from the Product and once as "price" from the Offer. One bad value is
	// one finding regardless of how many paths reach it.
	seen := make(map[string]bool)
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		key := code + "\x00" + value(data)
		if seen[key] {
			return
		}
		seen[key] = true
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data,
		})
	}

	for _, n := range g.Nodes {
		for _, prop := range urlProperties {
			for _, v := range g.Strs(n, prop) {
				if isResolvableURL(v) {
					continue
				}
				add(analyze.Warning, "structured-relative-url",
					"A structured-data URL is relative and will not resolve outside the page",
					map[string]any{"type": firstType(n), "property": prop, "value": v})
			}
		}
		for _, prop := range dateProperties {
			for _, v := range g.Strs(n, prop) {
				if isISODate(v) {
					continue
				}
				add(analyze.Warning, "structured-invalid-date",
					"A structured-data date is not in ISO 8601 format",
					map[string]any{"type": firstType(n), "property": prop, "value": v})
			}
		}
		for _, prop := range priceProperties {
			for _, raw := range g.Values(n, prop) {
				s, isString := raw.(string)
				// A JSON number is always well formed; only a string can carry a symbol,
				// a separator, or a range.
				if !isString || bareDecimalRe.MatchString(strings.TrimSpace(s)) {
					continue
				}
				add(analyze.Warning, "structured-malformed-price",
					"A structured-data price is not a bare decimal number",
					map[string]any{"type": firstType(n), "property": prop, "value": s})
			}
		}
	}
	issues = append(issues, priceMismatchIssues(p, g)...)
	return issues
}

// priceMismatchIssues compares the price in Product markup against the price rendered on the
// page. It only runs when the page shows exactly one distinct price: a sale price beside a
// struck-through original, or a variant selector that changes the price, puts several on the
// page, and there is then no single visible price for the markup to contradict.
func priceMismatchIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	onPage := distinctPagePrices(p)
	if len(onPage) != 1 {
		return nil
	}
	shown := onPage[0]
	var issues []analyze.Issue
	for _, n := range topLevelOfType(g, "Product") {
		marked, ok := normalizePrice(g.Str(n, "offers.price"))
		if !ok || marked == shown {
			continue
		}
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
			Code:    "structured-price-mismatch",
			Message: "The price in Product structured data differs from the price shown on the page",
			Data: map[string]any{
				"markup": formatPrice(marked),
				"page":   formatPrice(shown),
			},
		})
	}
	return issues
}

// distinctPagePrices returns the distinct prices rendered in the page body, using the same
// regexp the product candidate heuristic uses to recognize one. goquery's Text() walks every
// descendant text node with no tag exclusion, so JSON embedded in a <script> that lives in
// <body> is scraped as page text too; priceRe requires an adjacent currency symbol or code, so
// a raw JSON number such as "price":1999 does not match it, but a pre-formatted string some
// themes embed ("$19.99") does. The net effect can only ever add a distinct price, which makes
// this check fire less often, never more — an accidental safety margin, not something to
// "optimize" away.
func distinctPagePrices(p *crawler.Page) []float64 {
	seen := make(map[float64]bool)
	var out []float64
	for _, m := range priceRe.FindAllString(p.Doc.Find("body").Text(), -1) {
		v, ok := normalizePrice(m)
		if !ok {
			// A price we can't confidently parse (see disambiguateSeparators) must not be
			// silently dropped: doing so could turn a page that actually shows two prices
			// into one that looks like it shows a single, comparable price, which is
			// exactly the ambiguous case the mismatch check exists to stay silent on.
			// Treat "can't tell what this price is" the same as "there's more than one
			// price on this page" and give up on the whole page.
			return nil
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// normalizePrice extracts a comparable number from either a markup price or a rendered one.
// It decides what "." and "," mean rather than assuming a comma is always a thousands
// separator: comma-as-decimal is standard across DE/FR/NL/IT/ES, so "€19,99" and "$1,299" need
// to be told apart, not both read as thousands-grouped integers.
func normalizePrice(s string) (float64, bool) {
	digits := digitsRe.FindString(s)
	if digits == "" {
		return 0, false
	}
	// A trailing separator ("19.99." at a sentence end) is not part of the number.
	digits = strings.TrimSuffix(digits, ".")
	if digits == "" {
		return 0, false
	}
	normalized, ok := disambiguateSeparators(digits)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// disambiguateSeparators decides what role "." and "," play in a numeric string pulled from a
// price. Applied in order:
//  1. Both separators present: whichever occurs last is the decimal point; every instance of
//     the other is discarded as thousands grouping ("$1,299.00" and "€1.299,00" both become
//     "1299.00").
//  2. One kind of separator, appearing more than once: thousands grouping, strip all of it
//     ("1.234.567" and "1,234,567" both become "1234567").
//  3. One kind, appearing exactly once, followed by exactly two digits: a decimal point
//     ("€19,99" becomes "19.99").
//  4. One kind, appearing exactly once, followed by exactly three digits: genuinely ambiguous
//     — "1.299" and "1,299" are each plausibly 1299 or 1.299, and guessing risks the exact
//     false-positive storm this check exists to avoid, so this reports ok=false.
//  5. No separator: the digits are already a plain integer or decimal.
func disambiguateSeparators(digits string) (normalized string, ok bool) {
	hasDot := strings.Contains(digits, ".")
	hasComma := strings.Contains(digits, ",")

	switch {
	case hasDot && hasComma:
		if strings.LastIndex(digits, ".") > strings.LastIndex(digits, ",") {
			return strings.ReplaceAll(digits, ",", ""), true
		}
		return strings.Replace(strings.ReplaceAll(digits, ".", ""), ",", ".", 1), true
	case hasDot || hasComma:
		sep := "."
		if hasComma {
			sep = ","
		}
		if strings.Count(digits, sep) > 1 {
			return strings.ReplaceAll(digits, sep, ""), true
		}
		frac := digits[strings.Index(digits, sep)+1:]
		if len(frac) == 3 {
			return "", false
		}
		return strings.Replace(digits, sep, ".", 1), true
	default:
		return digits, true
	}
}

// formatPrice renders a normalized price for a finding's data.
func formatPrice(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// isResolvableURL reports whether a URL value stands on its own: absolute, protocol-relative,
// or a data URI.
func isResolvableURL(v string) bool {
	if v == "" {
		return true // absence is a different finding
	}
	if strings.HasPrefix(v, "//") || strings.HasPrefix(v, "data:") {
		return true
	}
	u, err := url.Parse(v)
	return err == nil && u.IsAbs()
}

// value returns a finding's offending value, used to de-duplicate findings that describe the
// same bad value reached by two different paths.
func value(data map[string]any) string {
	s, _ := data["value"].(string)
	return s
}

// isISODate reports whether a value parses as one of the ISO 8601 shapes schema.org accepts.
func isISODate(v string) bool {
	for _, layout := range isoDateLayouts {
		if _, err := time.Parse(layout, v); err == nil {
			return true
		}
	}
	return false
}
