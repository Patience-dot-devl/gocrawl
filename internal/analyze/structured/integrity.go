package structured

import (
	"sort"
	"strings"

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

// unresolvedIDIssues flags {"@id": ...} stubs whose target is not declared anywhere on the
// page. A dangling reference silently drops whatever the property was meant to convey — a
// publisher, a brand, a parent product.
func unresolvedIDIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	seen := make(map[string]bool)
	var issues []analyze.Issue
	for _, n := range g.Nodes {
		for key, v := range n.Props {
			if strings.HasPrefix(key, "@") {
				continue
			}
			for _, id := range referencedIDs(v) {
				if seen[id] {
					continue
				}
				if _, ok := g.Resolve(id); ok {
					continue
				}
				seen[id] = true
				issues = append(issues, analyze.Issue{
					Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
					Code:    "structured-unresolved-id",
					Message: "A JSON-LD @id reference points at a node that is not on the page",
					Data:    map[string]any{"id": id, "property": key, "type": firstType(n)},
				})
			}
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
