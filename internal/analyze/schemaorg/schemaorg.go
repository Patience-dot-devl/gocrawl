// Package schemaorg parses the JSON-LD embedded in an HTML page into a flat, addressable
// graph of typed nodes. It is a shared helper, not an analyzer: it emits no findings and is
// never registered. Both the structured and shopify analyzers read a page through it, so both
// see the same nodes and resolve @id references the same way. Nothing is cached: each caller
// parses the page again (three times per page today: once in structured, twice in shopify).
package schemaorg

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// jsonLDSelector matches the script elements that carry JSON-LD. Node.Block indexes into the
// elements this selector returns, in document order, so a caller that needs to correlate a
// node with the script that produced it (to attribute a block to the app that injected it,
// say) can re-run the same selector and index into it.
const jsonLDSelector = `script[type="application/ld+json"]`

// Node is one typed schema.org object, lifted out of whatever nesting it arrived in.
type Node struct {
	// Types holds the node's @type, always as a slice — schema.org permits a bare string or
	// an array, and normalizing here means no caller has to care which was used.
	Types []string
	// ID is the node's @id, empty when it declares none.
	ID string
	// Props is the node's raw decoded JSON object, @-prefixed keys included.
	Props map[string]any
	// Block is the index of the <script> element the node came from. Two nodes of the same
	// type in different blocks usually mean two sources competing to describe one page;
	// two in the same block are a deliberate modelling choice. Distinguishing the two is
	// the whole reason this field exists.
	Block int
	// Path is the node's dotted position within its block, e.g. "Product.offers". It lets a
	// finding name where in the markup a problem sits rather than only which type it hit.
	Path string
}

// Is reports whether the node declares the given @type.
func (n Node) Is(t string) bool {
	for _, have := range n.Types {
		if have == t {
			return true
		}
	}
	return false
}

// ParseError records a JSON-LD block that failed to decode.
type ParseError struct {
	Block int
	Err   string
}

// Graph is every typed node found on one page, in a stable order.
type Graph struct {
	Nodes []Node
	byID  map[string]int
}

// Parse extracts every JSON-LD block in the document and flattens it into a Graph. Blocks
// that fail to decode are returned as ParseErrors rather than aborting the parse, so one
// broken block from a misbehaving app does not hide the rest of a page's markup. Empty
// blocks are skipped silently — a theme that renders an empty script tag has no data, but it
// does not have a syntax error either.
func Parse(doc *goquery.Document) (Graph, []ParseError) {
	var g Graph
	var errs []ParseError
	if doc == nil {
		return g, nil
	}
	doc.Find(jsonLDSelector).Each(func(block int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			errs = append(errs, ParseError{Block: block, Err: err.Error()})
			return
		}
		g.walk(v, block, "")
	})
	g.index()
	return g, errs
}

// walk descends a decoded JSON-LD value, recording every object that declares an @type. The
// prefix carries the dotted path built so far; a typed object adopts its own type name as the
// path root when it is the first typed thing on the branch, so its children read as
// "Product.offers" rather than just "offers".
func (g *Graph) walk(v any, block int, prefix string) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			g.walk(item, block, prefix)
		}
	case map[string]any:
		path := prefix
		if types := asStrings(t["@type"]); len(types) > 0 {
			if path == "" {
				path = types[0]
			}
			id, _ := t["@id"].(string)
			g.Nodes = append(g.Nodes, Node{Types: types, ID: id, Props: t, Block: block, Path: path})
		}
		// Go randomizes map iteration order, and a JSON object has no order of its own
		// to fall back on. Without sorting, sibling typed children land in g.Nodes in a
		// different sequence on every run, and that churn surfaces as a spurious diff
		// on a page that never actually changed.
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := t[key]
			if strings.HasPrefix(key, "@") {
				// @graph members are siblings of the node that declared the graph, not
				// properties of it, so they keep the current prefix. Every other @-key
				// (@type, @id, @context) is metadata with nothing to descend into.
				if key == "@graph" {
					g.walk(val, block, prefix)
				}
				continue
			}
			g.walk(val, block, joinPath(path, key))
		}
	}
}

// index builds the @id lookup once the walk is complete, so that a reference appearing before
// its target still resolves.
func (g *Graph) index() {
	for i, n := range g.Nodes {
		if n.ID == "" {
			continue
		}
		if _, dup := g.byID[n.ID]; dup {
			continue // first declaration wins
		}
		if g.byID == nil {
			g.byID = make(map[string]int)
		}
		g.byID[n.ID] = i
	}
}

// OfType returns every node declaring the given @type, in a stable order.
func (g Graph) OfType(t string) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.Is(t) {
			out = append(out, n)
		}
	}
	return out
}

// HasType reports whether any node on the page declares any one of the given types.
func (g Graph) HasType(types ...string) bool {
	for _, t := range types {
		if len(g.OfType(t)) > 0 {
			return true
		}
	}
	return false
}

// Types returns every @type on the page, de-duplicated, in a stable order.
func (g Graph) Types() []string {
	seen := make(map[string]bool)
	var out []string
	for _, n := range g.Nodes {
		for _, t := range n.Types {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// Resolve returns the node declaring the given @id.
func (g Graph) Resolve(id string) (Node, bool) {
	i, ok := g.byID[id]
	if !ok {
		return Node{}, false
	}
	return g.Nodes[i], true
}

// joinPath appends a property name to a dotted path.
func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// asStrings normalizes a schema.org value that may be a bare string or an array of strings.
func asStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
