# Structured-Data Depth and Shopify Analyzer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give gocrawl the ability to judge a site's structured data against Google's rich-result requirements, catch markup that contradicts itself or its page, and report template-aware schema coverage for Shopify stores.

**Architecture:** A new pure package `internal/analyze/schemaorg` parses every JSON-LD block on a page into a flat, addressable node graph. Two analyzers consume it: `structured` (platform-neutral eligibility, integrity, and site-level rollup) and a new `shopify` CMS analyzer (detection, template classification, Shopify-specific pitfalls). No engine changes.

**Tech Stack:** Go 1.26, `github.com/PuerkitoBio/goquery` for DOM access, `encoding/json` for JSON-LD, standard `testing` package. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-09-14-structured-data-shopify-design.md`

## Global Constraints

> **Correction applied mid-execution (Task 5).** The repo already has
> `TestAllAnalyzerCodesHaveExplanations` in `internal/report/explanations_test.go`, a static
> contract test that fails the whole-repo suite the moment an analyzer emits a code with no
> entry in `internal/report/explanations.go`. That means **every task that introduces a new
> issue code must add its explanation entry in the same commit** — the suite will not go green
> otherwise. Tasks 7, 12 and 16 therefore do NOT add entries for codes an earlier task already
> shipped; re-adding them is a duplicate-map-key compile error. Those three tasks are
> verification and documentation only, plus entries for any code not yet covered.


- Module path is `github.com/Patience-dot-devl/gocrawl`. Go 1.26+.
- Analyzers must be **pure**: read a `crawler.Result`, return `[]analyze.Issue`. Never fetch, never mutate shared state, never print. The single exception in this plan is the opt-in `/products.json` probe in Task 15, which uses the injected `crawler.Fetcher` exactly like the `sitemap` and `wordpress` analyzers do.
- `schemaorg` is **not** an analyzer. It emits no `analyze.Issue` and is never registered. It is a shared helper, the same role `internal/analyze/seaurl` plays for UTM parsing.
- Issue `Code` values are part of the report contract. Never rename an existing code. Every **new** code must get an entry in `internal/report/explanations.go`.
- `Analyzer` implementations must be safe for sequential reuse — keep per-crawl accumulator state local to `Analyze`, never on the struct.
- Run `gofmt -l .` and `go vet ./...` before every commit. CI runs `go test -race ./...` and golangci-lint (`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `misspell`).
- Every exported identifier gets a doc comment. Match the surrounding code's comment density — this codebase explains *why* a threshold or heuristic exists, not just what the code does.
- Three deliberate behaviour changes are expected in Phase 1 and are called out in the tasks that cause them. Any *other* change to existing `structured` output is a bug.

---

## File Structure

**New:**

| File | Responsibility |
| --- | --- |
| `internal/analyze/schemaorg/schemaorg.go` | `Node`, `Graph`, `ParseError`, `Parse`, type accessors |
| `internal/analyze/schemaorg/path.go` | Dotted-path resolution, `@id` dereferencing, typed property readers |
| `internal/analyze/schemaorg/schemaorg_test.go` | Tests for both files |
| `internal/analyze/structured/eligibility.go` | Required / recommended / merchant field tables and the checks that read them |
| `internal/analyze/structured/integrity.go` | Duplicate, conflict, price-mismatch, unresolved-`@id`, URL, date, price-format checks |
| `internal/analyze/structured/candidates.go` | The four existing `*-candidate` heuristics, moved verbatim |
| `internal/analyze/structured/rollup.go` | Site-level accumulator for recommended / merchant gaps |
| `internal/analyze/shopify/shopify.go` | Analyzer wiring, detection, site-level assembly |
| `internal/analyze/shopify/template.go` | URL-path template classification and the expected-schema table |
| `internal/analyze/shopify/schema.go` | App fingerprinting, conflict / client-injection / variant checks |
| `internal/analyze/shopify/seo.go` | Phase 3 general Shopify SEO checks |
| `internal/analyze/shopify/*_test.go` | One test file per source file above |

**Modified:**

| File | Change |
| --- | --- |
| `internal/analyze/analyze.go` | Add exported `SiteBase(result)` helper |
| `internal/analyze/wordpress/wordpress.go` | Drop its private `siteBase`, call `analyze.SiteBase` |
| `internal/analyze/structured/structured.go` | Shrinks to analyzer wiring; parsing moves to `schemaorg`, checks move to sibling files |
| `internal/analyze/structured/structured_test.go` | Behaviour-pinning tests first, then updates for the three deliberate changes |
| `internal/runner/runner.go` | Register `shopify` after `wordpress` |
| `internal/report/explanations.go` | Entries for every new code |
| `docs/analyzers.md` | Expanded `structured` table, new `shopify` section |
| `CLAUDE.md` | `shopify` in the registered-analyzer list, `schemaorg` in the package map |

---

# Phase 1 — `schemaorg` and `structured` depth

### Task 1: `schemaorg` graph extraction

**Files:**
- Create: `internal/analyze/schemaorg/schemaorg.go`
- Create: `internal/analyze/schemaorg/schemaorg_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Node struct { Types []string; ID string; Props map[string]any; Block int; Path string }`
  - `type ParseError struct { Block int; Err string }`
  - `type Graph struct { Nodes []Node; byID map[string]int }`
  - `func Parse(doc *goquery.Document) (Graph, []ParseError)`
  - `func (g Graph) OfType(t string) []Node`
  - `func (g Graph) HasType(types ...string) bool`
  - `func (g Graph) Types() []string`
  - `func (g Graph) Resolve(id string) (Node, bool)`
  - `func (n Node) Is(t string) bool`

- [ ] **Step 1: Write the failing test**

Create `internal/analyze/schemaorg/schemaorg_test.go`:

```go
package schemaorg_test

import (
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/PuerkitoBio/goquery"
)

func parse(t *testing.T, html string) (schemaorg.Graph, []schemaorg.ParseError) {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	return schemaorg.Parse(doc)
}

func TestParseFlattensNestedObjects(t *testing.T) {
	g, errs := parse(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","name":"Tee",
		 "brand":{"@type":"Brand","name":"Acme"},
		 "offers":{"@type":"Offer","price":"19.99"}}
	</script></head><body></body></html>`)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes (Product, Brand, Offer), got %d", len(g.Nodes))
	}
	offers := g.OfType("Offer")
	if len(offers) != 1 {
		t.Fatalf("expected 1 Offer node, got %d", len(offers))
	}
	if offers[0].Path != "Product.offers" {
		t.Errorf("expected Offer path Product.offers, got %q", offers[0].Path)
	}
	if offers[0].Block != 0 {
		t.Errorf("expected Offer in block 0, got %d", offers[0].Block)
	}
}

func TestParseDescendsGraphAndArrays(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@graph":[{"@type":"WebSite"},{"@type":["BreadcrumbList","ItemList"]}]}
	</script></head><body></body></html>`)
	if !g.HasType("WebSite") || !g.HasType("BreadcrumbList") || !g.HasType("ItemList") {
		t.Errorf("expected WebSite, BreadcrumbList and ItemList, got %v", g.Types())
	}
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes from @graph, got %d", len(g.Nodes))
	}
}

func TestParseRecordsBlockIndex(t *testing.T) {
	g, _ := parse(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"A"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"B"}</script>
	</head><body></body></html>`)
	products := g.OfType("Product")
	if len(products) != 2 {
		t.Fatalf("expected 2 Product nodes, got %d", len(products))
	}
	if products[0].Block != 0 || products[1].Block != 1 {
		t.Errorf("expected blocks 0 and 1, got %d and %d", products[0].Block, products[1].Block)
	}
}

func TestParseReportsInvalidBlock(t *testing.T) {
	g, errs := parse(t, `<html><head>
		<script type="application/ld+json">{ not json }</script>
		<script type="application/ld+json">{"@type":"Organization","name":"Acme"}</script>
	</head><body></body></html>`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(errs))
	}
	if errs[0].Block != 0 {
		t.Errorf("expected the error on block 0, got %d", errs[0].Block)
	}
	if !g.HasType("Organization") {
		t.Error("expected the valid block to still be parsed")
	}
}

func TestParseSkipsEmptyBlocks(t *testing.T) {
	_, errs := parse(t, `<html><head><script type="application/ld+json">   </script></head><body></body></html>`)
	if len(errs) != 0 {
		t.Errorf("an empty block is not an error, got %v", errs)
	}
}

func TestResolveByID(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@graph":[{"@type":"Organization","@id":"https://x.test/#org","name":"Acme"},
		           {"@type":"WebSite","publisher":{"@id":"https://x.test/#org"}}]}
	</script></head><body></body></html>`)
	n, ok := g.Resolve("https://x.test/#org")
	if !ok {
		t.Fatal("expected to resolve the Organization by @id")
	}
	if !n.Is("Organization") {
		t.Errorf("expected an Organization node, got %v", n.Types)
	}
}

func TestTypesAreDeduplicatedInDocumentOrder(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		[{"@type":"Product","name":"A"},{"@type":"Product","name":"B"},{"@type":"Organization","name":"C"}]
	</script></head><body></body></html>`)
	got := g.Types()
	if len(got) != 2 || got[0] != "Product" || got[1] != "Organization" {
		t.Errorf("expected [Product Organization], got %v", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/analyze/schemaorg/ -run TestParse -v`
Expected: FAIL — the package does not exist yet (`no required module provides package`).

- [ ] **Step 3: Write the implementation**

Create `internal/analyze/schemaorg/schemaorg.go`:

```go
// Package schemaorg parses the JSON-LD embedded in an HTML page into a flat, addressable
// graph of typed nodes. It is a shared helper, not an analyzer: it emits no findings and is
// never registered. Both the structured and shopify analyzers read a page through it, so
// JSON-LD is parsed once per page and @id references resolve against the same graph.
package schemaorg

import (
	"encoding/json"
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

// Graph is every typed node found on one page, in document order.
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
		for key, val := range t {
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

// OfType returns every node declaring the given @type, in document order.
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

// Types returns every @type on the page, de-duplicated, in document order.
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/analyze/schemaorg/ -v`
Expected: PASS, all seven tests.

- [ ] **Step 5: Format, vet, commit**

```bash
gofmt -w internal/analyze/schemaorg/
go vet ./internal/analyze/schemaorg/
git add internal/analyze/schemaorg/
git commit -m "feat(schemaorg): flatten page JSON-LD into an addressable node graph"
```

---

### Task 2: Dotted-path resolution and typed property readers

**Files:**
- Create: `internal/analyze/schemaorg/path.go`
- Modify: `internal/analyze/schemaorg/schemaorg_test.go` (append)

**Interfaces:**
- Consumes: `Node`, `Graph`, `Resolve` from Task 1.
- Produces:
  - `func (g Graph) Value(n Node, path string) (any, bool)`
  - `func (g Graph) Str(n Node, path string) string`
  - `func (g Graph) Strs(n Node, path string) []string`
  - `func (g Graph) NodesAt(n Node, path string) []Node`
  - `func (g Graph) HasValue(n Node, path string) bool`

- [ ] **Step 1: Write the failing test**

Append to `internal/analyze/schemaorg/schemaorg_test.go`:

```go
func TestStrResolvesDottedPath(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD"}}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	if got := g.Str(p, "offers.price"); got != "19.99" {
		t.Errorf("expected 19.99, got %q", got)
	}
	if got := g.Str(p, "offers.priceCurrency"); got != "USD" {
		t.Errorf("expected USD, got %q", got)
	}
	if got := g.Str(p, "offers.availability"); got != "" {
		t.Errorf("expected empty for an absent path, got %q", got)
	}
}

func TestStrCoercesNumbers(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":19.99}}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	if got := g.Str(p, "offers.price"); got != "19.99" {
		t.Errorf("expected a numeric price rendered as 19.99, got %q", got)
	}
}

func TestPathFollowsIDReference(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@graph":[{"@type":"Brand","@id":"https://x.test/#brand","name":"Acme"},
		           {"@type":"Product","name":"Tee","brand":{"@id":"https://x.test/#brand"}}]}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	if got := g.Str(p, "brand.name"); got != "Acme" {
		t.Errorf("expected the @id reference to resolve to Acme, got %q", got)
	}
}

func TestPathTraversesArrays(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":[{"@type":"Offer","price":"10.00"},{"@type":"Offer","price":"20.00"}]}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	got := g.Strs(p, "offers.price")
	if len(got) != 2 || got[0] != "10.00" || got[1] != "20.00" {
		t.Errorf("expected both offer prices, got %v", got)
	}
}

func TestHasValueTreatsEmptyAsAbsent(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","sku":"","description":"  ","image":[],"brand":{"name":"Acme"}}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	for _, path := range []string{"sku", "description", "image", "gtin"} {
		if g.HasValue(p, path) {
			t.Errorf("expected %q to count as absent", path)
		}
	}
	if !g.HasValue(p, "brand") {
		t.Error("expected a populated brand object to count as present")
	}
}

func TestNodesAtReturnsTypedChildren(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"ProductGroup","name":"Tee","hasVariant":[{"@type":"Product","name":"S"},{"@type":"Product","name":"M"}]}
	</script></head><body></body></html>`)
	pg := g.OfType("ProductGroup")[0]
	if got := g.NodesAt(pg, "hasVariant"); len(got) != 2 {
		t.Errorf("expected 2 variant nodes, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/schemaorg/ -run 'TestStr|TestPath|TestHasValue|TestNodesAt' -v`
Expected: FAIL — `g.Str undefined`, `g.Strs undefined`, `g.HasValue undefined`, `g.NodesAt undefined`.

- [ ] **Step 3: Write the implementation**

Create `internal/analyze/schemaorg/path.go`:

```go
package schemaorg

import (
	"strconv"
	"strings"
)

// Value returns the first value at a dotted path below n, e.g. "offers.price". Path
// resolution absorbs the three shapes schema.org allows interchangeably at every step: a
// bare value, an array of values, and a bare {"@id": ...} reference to a node declared
// elsewhere on the page. Callers get to write "offers.price" and never branch on which was
// used.
func (g Graph) Value(n Node, path string) (any, bool) {
	vals := g.values(n.Props, strings.Split(path, "."))
	if len(vals) == 0 {
		return nil, false
	}
	return vals[0], true
}

// values walks the remaining path segments over v, collecting every value reached. It returns
// all matches rather than the first so that a property holding an array (two Offers, several
// images) resolves completely.
func (g Graph) values(v any, segs []string) []any {
	if len(segs) == 0 {
		if v == nil {
			return nil
		}
		return []any{v}
	}
	switch t := v.(type) {
	case []any:
		var out []any
		for _, item := range t {
			out = append(out, g.values(item, segs)...)
		}
		return out
	case map[string]any:
		next, ok := t[segs[0]]
		if !ok {
			// A bare {"@id": ...} stub stands in for a node declared elsewhere; follow it
			// and retry the segment there. Guarded on the key being absent locally so a
			// node that carries both an @id and the property keeps its own value.
			if id, isRef := t["@id"].(string); isRef {
				if target, found := g.Resolve(id); found && !sameObject(target.Props, t) {
					return g.values(target.Props, segs)
				}
			}
			return nil
		}
		return g.values(next, segs[1:])
	}
	return nil
}

// sameObject reports whether two decoded objects are the same map, guarding the @id follow in
// values against a node whose @id resolves back to itself.
func sameObject(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// Str returns the first value at path rendered as a string. Numbers are formatted without a
// trailing ".0" so that a price written as 19.99 and one written as "19.99" compare equal.
func (g Graph) Str(n Node, path string) string {
	v, ok := g.Value(n, path)
	if !ok {
		return ""
	}
	return toString(v)
}

// Strs returns every value at path rendered as a string, skipping values that have no
// sensible scalar form (nested objects, nulls).
func (g Graph) Strs(n Node, path string) []string {
	var out []string
	for _, v := range g.values(n.Props, strings.Split(path, ".")) {
		if s := toString(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// NodesAt returns the typed nodes reachable at path below n. It matches against the parsed
// graph by object identity of the underlying map, so a value reached through an @id reference
// comes back as the node that declared it.
func (g Graph) NodesAt(n Node, path string) []Node {
	var out []Node
	for _, v := range g.values(n.Props, strings.Split(path, ".")) {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for _, cand := range g.Nodes {
			if sameObject(cand.Props, m) && len(asStrings(m["@type"])) > 0 {
				out = append(out, cand)
				break
			}
		}
	}
	return out
}

// HasValue reports whether n carries a non-empty value at path. Empty strings, whitespace,
// empty arrays, and empty objects all count as absent — markup that declares a property and
// leaves it blank is no more useful to a search engine than omitting it.
func (g Graph) HasValue(n Node, path string) bool {
	for _, v := range g.values(n.Props, strings.Split(path, ".")) {
		switch x := v.(type) {
		case nil:
		case string:
			if strings.TrimSpace(x) != "" {
				return true
			}
		case []any:
			if len(x) > 0 {
				return true
			}
		case map[string]any:
			if len(x) > 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// toString renders a decoded JSON scalar as a string, returning "" for values with no scalar
// form.
func toString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/analyze/schemaorg/ -v`
Expected: PASS, all thirteen tests.

- [ ] **Step 5: Format, vet, commit**

```bash
gofmt -w internal/analyze/schemaorg/
go vet ./internal/analyze/schemaorg/
git add internal/analyze/schemaorg/
git commit -m "feat(schemaorg): resolve dotted paths through arrays and @id references"
```

---

### Task 3: Refactor `structured` onto `schemaorg`

This task changes no behaviour except the three deliberate changes named in the spec. Pin the current output first, then swap the parser under it.

**Files:**
- Modify: `internal/analyze/structured/structured.go`
- Create: `internal/analyze/structured/candidates.go`
- Modify: `internal/analyze/structured/structured_test.go`
- Modify: `internal/analyze/analyze.go`
- Modify: `internal/analyze/wordpress/wordpress.go:544-556`

**Interfaces:**
- Consumes: `schemaorg.Parse`, `Graph.Types`, `Graph.HasType`, `Graph.OfType`, `Graph.HasValue` from Tasks 1–2.
- Produces:
  - `func analyze.SiteBase(result *crawler.Result) string`
  - unexported in `structured`: `func candidateIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue`

- [ ] **Step 1: Write the pinning tests**

Append to `internal/analyze/structured/structured_test.go`. These assert the three deliberate changes *by name*, so a reviewer can see the intent rather than inferring it from a diff:

```go
// Pinned: the three deliberate behaviour changes from the phase 1 refactor. Each of these
// documents a case whose result intentionally differs from the pre-schemaorg analyzer.

func TestStructuredNestedTypesAreReported(t *testing.T) {
	// Deliberate change 2: the old collectTypes descended only into @graph, so a nested
	// Offer never appeared in the reported type list.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99",
		 "priceCurrency":"USD","availability":"https://schema.org/InStock"},"image":"https://x.test/a.jpg"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-data")
	if !ok {
		t.Fatal("expected structured-data issue")
	}
	types, _ := is.Data["types"].([]string)
	var sawOffer bool
	for _, ty := range types {
		if ty == "Offer" {
			sawOffer = true
		}
	}
	if !sawOffer {
		t.Errorf("expected the nested Offer in the reported types, got %v", types)
	}
}

func TestStructuredNestedOfferSuppressesProductCandidate(t *testing.T) {
	// Deliberate change 2, second half: a nested Offer now suppresses the product candidate.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"WebPage","mainEntity":{"@type":"Offer","price":"19.99"}}
	</script></head><body><p>$19.99</p><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate when a nested Offer is present")
	}
}

func TestStructuredBareOfferNoLongerRequired(t *testing.T) {
	// Deliberate change 1: Offer left the top-level required-field table.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Offer","url":"https://x.test/p"}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("a bare top-level Offer is no longer checked for required fields")
	}
}
```

Also update the existing `TestStructuredValidProductNoViolation`, which asserts a name-only `Product` raises nothing. Deliberate change 3 makes that false. Replace its body:

```go
func TestStructuredValidProductNoViolation(t *testing.T) {
	// Deliberate change 3: Product's required tier is now name, image, offers.price,
	// offers.priceCurrency and offers.availability, so a complete Product needs all five.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","name":"Widget","image":"https://x.test/w.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("did not expect structured-missing-required for a complete Product")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/structured/ -v`
Expected: `TestStructuredNestedTypesAreReported`, `TestStructuredNestedOfferSuppressesProductCandidate` and `TestStructuredBareOfferNoLongerRequired` FAIL. `TestStructuredValidProductNoViolation` still passes (the new fixture is a superset), which is fine — it is being made future-proof, not driving this step.

- [ ] **Step 3: Add the shared `SiteBase` helper**

Append to `internal/analyze/analyze.go`:

```go
// SiteBase returns the scheme://host of the crawl, derived from the seed or, failing that,
// from the first crawled page's final URL. Analyzers that aggregate a finding across the
// whole crawl — a server configuration, a CMS fingerprint, a template-wide schema gap — use
// it as the issue URL, so one site-wide fact does not repeat on every page of the report.
func SiteBase(result *crawler.Result) string {
	if u, err := url.Parse(result.Seed); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	for _, p := range result.Pages {
		if u, err := url.Parse(p.FinalURL); err == nil && u.Host != "" {
			return u.Scheme + "://" + u.Host
		}
	}
	return result.Seed
}
```

Add `"net/url"` to the import block. Then delete the private `siteBase` from `internal/analyze/wordpress/wordpress.go` (lines 544-556) and replace its two call sites with `analyze.SiteBase(result)`.

- [ ] **Step 4: Run the wordpress tests to confirm the swap is neutral**

Run: `go test ./internal/analyze/wordpress/ ./internal/analyze/ -v`
Expected: PASS, unchanged.

- [ ] **Step 5: Move the candidate heuristics to their own file**

Create `internal/analyze/structured/candidates.go`. Move `articleTypes`, `minArticleWords`, `priceRe`, `cartSignalRe`, `videoHostRe`, `candidateIssues`, `hasBreadcrumbMarkup`, `hasProductSignal`, `hasArticleSignal` and `findVideoEmbed` out of `structured.go` **verbatim**, adding only the `package structured` clause and the imports they need (`regexp`, `strings`, `github.com/PuerkitoBio/goquery`, plus the two internal packages). Then change `candidateIssues`'s signature from `(p *crawler.Page, types map[string]bool)` to `(p *crawler.Page, g schemaorg.Graph)` and replace each `types["X"]` lookup with `g.HasType("X")`:

```go
	if !g.HasType("BreadcrumbList") {
	if !g.HasType("Product", "Offer") {
	hasArticleType := g.HasType("Article", "NewsArticle", "BlogPosting", "TechArticle", "Report")
	if !g.HasType("VideoObject") {
```

`articleTypes` and the loop that walked it are now redundant — delete both.

- [ ] **Step 6: Rewrite `structured.go` on top of the graph**

Replace the body of `internal/analyze/structured/structured.go` with:

```go
// Package structured extracts and reports on JSON-LD structured data (schema.org): which
// types a page declares, whether they carry the fields their rich results require, whether
// the markup contradicts itself or the page it sits on, and which pages look like they are
// missing markup they should have.
package structured

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer extracts JSON-LD blocks and reports on the structured data they carry.
type Analyzer struct{}

// New returns a structured-data analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "structured" }
func (Analyzer) Description() string {
	return "JSON-LD structured data: schema.org type reporting, rich-result field eligibility, markup integrity, and missing-markup candidates"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	issues := analyze.EachPage(result, a.analyzePage)
	return issues
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	g, parseErrs := schemaorg.Parse(p.Doc)

	var issues []analyze.Issue
	for _, e := range parseErrs {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
			Code: "structured-invalid-jsonld", Message: "JSON-LD block is not valid JSON",
			Data: map[string]any{"error": e.Err},
		})
	}
	issues = append(issues, requiredIssues(p, g)...)
	issues = append(issues, candidateIssues(p, g)...)

	// A page with no parsed nodes and no failed blocks has no JSON-LD at all. Checking the
	// node count rather than the script count means a page whose only block is empty or
	// broken is reported through the more specific code, not as "none".
	if len(g.Nodes) == 0 && len(parseErrs) == 0 {
		return append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Info,
			Code: "structured-none", Message: "Page has no JSON-LD structured data",
		})
	}
	if types := g.Types(); len(types) > 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Info,
			Code: "structured-data", Message: "JSON-LD structured data found",
			Data: map[string]any{"types": types},
		})
	}
	return issues
}
```

`requiredIssues` lands in Task 4; for this step, add a temporary shim at the bottom of `structured.go` that preserves the old behaviour so the package compiles and the pinning tests are meaningful:

```go
// requiredIssues reports typed nodes missing the fields their rich result requires. Task 4
// replaces this with the table-driven eligibility check in eligibility.go.
func requiredIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	var issues []analyze.Issue
	for _, n := range g.Nodes {
		for _, ty := range n.Types {
			req, known := legacyRequired[ty]
			if !known {
				continue
			}
			var missing []string
			for _, f := range req {
				if !g.HasValue(n, f) {
					missing = append(missing, f)
				}
			}
			if len(missing) > 0 {
				issues = append(issues, analyze.Issue{
					Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
					Code: "structured-missing-required", Message: "Structured-data object is missing required schema.org fields",
					Data: map[string]any{"type": ty, "missing": missing},
				})
			}
		}
	}
	return issues
}

// legacyRequired is the pre-eligibility required-field table, carried unchanged except that
// Offer has left it — its fields are now reached through Product via a dotted path.
var legacyRequired = map[string][]string{
	"Product":        {"name"},
	"Article":        {"headline"},
	"NewsArticle":    {"headline"},
	"BlogPosting":    {"headline"},
	"Recipe":         {"name"},
	"Event":          {"name", "startDate"},
	"Organization":   {"name"},
	"LocalBusiness":  {"name"},
	"Person":         {"name"},
	"BreadcrumbList": {"itemListElement"},
	"FAQPage":        {"mainEntity"},
	"VideoObject":    {"name", "thumbnailUrl"},
}
```

Delete the now-unused `validateRequired`, `missingReq`, `hasField`, `collectTypes`, `asStrings`, `toSet`, `dedupe` and `requiredFields` from `structured.go` — every one of them now lives in `schemaorg` or is superseded.

- [ ] **Step 7: Run the full structured suite**

Run: `go test ./internal/analyze/structured/ -v`
Expected: PASS, including the three new pinning tests. `TestStructuredValidProductNoViolation` passes against the legacy table too — Task 4 is what makes it load-bearing.

- [ ] **Step 8: Run the whole suite to catch downstream breakage**

Run: `go test ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
gofmt -w internal/analyze/
git add internal/analyze/
git commit -m "refactor(structured): read pages through the schemaorg graph

Moves JSON-LD parsing into the shared schemaorg package and the candidate
heuristics into their own file. Three intended behaviour changes: nested
types are now reported and suppress candidates, a bare top-level Offer is
no longer required-field checked, and analyze.SiteBase replaces the
wordpress analyzer's private copy."
```

---

### Task 4: Rich-result eligibility tiers and the site-level rollup

**Files:**
- Create: `internal/analyze/structured/eligibility.go`
- Create: `internal/analyze/structured/rollup.go`
- Create: `internal/analyze/structured/eligibility_test.go`
- Modify: `internal/analyze/structured/structured.go`

**Interfaces:**
- Consumes: `schemaorg.Graph`, `Graph.HasValue`, `Graph.Nodes`, `analyze.SiteBase` from Tasks 1–3.
- Produces:
  - `func requiredIssues(p *crawler.Page, g schemaorg.Graph, roll *rollup) []analyze.Issue` (replaces the Task 3 shim, gains the `roll` parameter)
  - `type rollup struct{...}`, `func newRollup() *rollup`, `func (r *rollup) add(code, typ string, missing []string, pageURL string)`, `func (r *rollup) issues(base string) []analyze.Issue`
  - New codes: `structured-missing-recommended`, `structured-missing-merchant`

- [ ] **Step 1: Write the failing tests**

Create `internal/analyze/structured/eligibility_test.go`:

```go
package structured_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// pages builds a Result from several (url, html) pairs so rollup behaviour can be asserted
// across a crawl rather than a single page.
func pages(t *testing.T, seed string, urlHTML map[string]string) *crawler.Result {
	t.Helper()
	res := &crawler.Result{Seed: seed}
	for u, html := range urlHTML {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("parse %s: %v", u, err)
		}
		res.Pages = append(res.Pages, &crawler.Page{
			FinalURL: u, StatusCode: 200, ContentType: "text/html", Doc: doc,
		})
	}
	return res
}

func findAll(issues []analyze.Issue, code string) []analyze.Issue {
	var out []analyze.Issue
	for _, is := range issues {
		if is.Code == code {
			out = append(out, is)
		}
	}
	return out
}

// completeProduct carries every required Product field and nothing else, so a test can add
// exactly one tier's worth of fields and assert which code fires.
const completeProduct = `{"@context":"https://schema.org","@type":"Product","name":"Tee",
 "image":"https://shop.test/t.jpg",
 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}`

func TestProductMissingRequiredFieldIsPerPage(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for a Product with no offers")
	}
	missing, _ := is.Data["missing"].([]string)
	want := map[string]bool{"offers.price": true, "offers.priceCurrency": true, "offers.availability": true}
	if len(missing) != 3 {
		t.Fatalf("expected 3 missing offer fields, got %v", missing)
	}
	for _, m := range missing {
		if !want[m] {
			t.Errorf("unexpected missing field %q", m)
		}
	}
	if is.URL != "https://example.com/" {
		t.Errorf("expected a per-page URL, got %q", is.URL)
	}
}

func TestCompleteProductRaisesNoRequiredGap(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">`+completeProduct+`</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("a Product with all five required fields should raise no required gap")
	}
}

func TestRecommendedGapRollsUpAcrossPages(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + completeProduct + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{
		"https://shop.test/products/a": html,
		"https://shop.test/products/b": html,
		"https://shop.test/products/c": html,
	})
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-recommended")
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 rolled-up issue for 3 pages, got %d", len(got))
	}
	is := got[0]
	if is.Severity != analyze.Info {
		t.Errorf("expected info severity, got %q", is.Severity)
	}
	if is.URL != "https://shop.test" {
		t.Errorf("expected the site base as the issue URL, got %q", is.URL)
	}
	if is.Data["type"] != "Product" {
		t.Errorf("expected type Product, got %v", is.Data["type"])
	}
	if is.Data["pages"] != 3 {
		t.Errorf("expected pages=3, got %v", is.Data["pages"])
	}
	missing, _ := is.Data["missing"].(map[string]int)
	if missing["brand"] != 3 || missing["aggregateRating"] != 3 {
		t.Errorf("expected brand and aggregateRating missing on all 3 pages, got %v", missing)
	}
	examples, _ := is.Data["examples"].([]string)
	if len(examples) != 3 {
		t.Errorf("expected 3 example URLs, got %v", examples)
	}
}

func TestRollupCapsExamplesAtFive(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + completeProduct + `</script></head><body></body></html>`
	urls := map[string]string{}
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		urls["https://shop.test/products/"+s] = html
	}
	res := pages(t, "https://shop.test", urls)
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-recommended")
	if len(got) != 1 {
		t.Fatalf("expected 1 rolled-up issue, got %d", len(got))
	}
	if got[0].Data["pages"] != 7 {
		t.Errorf("expected pages=7, got %v", got[0].Data["pages"])
	}
	examples, _ := got[0].Data["examples"].([]string)
	if len(examples) != 5 {
		t.Errorf("expected examples capped at 5, got %d", len(examples))
	}
}

func TestMerchantGapUsesItsOwnCode(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + completeProduct + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := structured.New().Analyze(context.Background(), res)
	merchant := findAll(issues, "structured-missing-merchant")
	if len(merchant) != 1 {
		t.Fatalf("expected 1 merchant-gap issue, got %d", len(merchant))
	}
	missing, _ := merchant[0].Data["missing"].(map[string]int)
	for _, f := range []string{"gtin|gtin8|gtin12|gtin13|gtin14|mpn", "priceValidUntil", "offers.shippingDetails", "hasMerchantReturnPolicy"} {
		if missing[f] != 1 {
			t.Errorf("expected %q reported missing once, got %d", f, missing[f])
		}
	}
	// A merchant field must never appear under the recommended code.
	for _, is := range findAll(issues, "structured-missing-recommended") {
		m, _ := is.Data["missing"].(map[string]int)
		if _, leaked := m["hasMerchantReturnPolicy"]; leaked {
			t.Error("merchant fields must not leak into the recommended rollup")
		}
	}
}

func TestAnyOfMerchantFieldSatisfiedByOneAlternative(t *testing.T) {
	html := `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg","mpn":"AC-1",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-merchant")
	if len(got) != 1 {
		t.Fatalf("expected 1 merchant-gap issue, got %d", len(got))
	}
	missing, _ := got[0].Data["missing"].(map[string]int)
	if _, present := missing["gtin|gtin8|gtin12|gtin13|gtin14|mpn"]; present {
		t.Error("mpn alone should satisfy the gtin-or-mpn identifier requirement")
	}
}

func TestNestedListProductsAreExemptFromEligibility(t *testing.T) {
	// A Shopify collection page lists products with only a name and a URL. Required-field
	// checking those would put a warning on every tile of every collection page.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"ItemList","itemListElement":[
			{"@type":"Product","name":"A","url":"https://shop.test/products/a"},
			{"@type":"Product","name":"B","url":"https://shop.test/products/b"}]}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("products nested in an ItemList must be exempt from eligibility checks")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/structured/ -run 'Product|Recommended|Rollup|Merchant|AnyOf|Nested' -v`
Expected: FAIL — the required table still holds only `name` for Product, and neither rollup code exists.

- [ ] **Step 3: Write the rollup accumulator**

Create `internal/analyze/structured/rollup.go`:

```go
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
```

- [ ] **Step 4: Write the eligibility tables**

Create `internal/analyze/structured/eligibility.go`:

```go
package structured

import (
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// fieldSpec splits a type's properties into the three tiers that matter to a search engine.
// A field name may be an any-of group written with "|" separators ("gtin|mpn"), satisfied by
// any one alternative — schema.org offers several interchangeable spellings for the same
// requirement and demanding a specific one would be wrong.
type fieldSpec struct {
	// required fields block rich-result eligibility outright when absent.
	required []string
	// recommended fields do not block eligibility but degrade the result when absent.
	recommended []string
	// merchant fields feed Google's Shopping and free-listing surfaces. Product only.
	merchant []string
}

// eligibility is a curated subset of schema.org tied to documented rich-result requirements,
// not a full vocabulary. Adding a type here is the whole cost of covering a new rich result.
var eligibility = map[string]fieldSpec{
	"Product": {
		required:    []string{"name", "image", "offers.price", "offers.priceCurrency", "offers.availability"},
		recommended: []string{"brand", "sku", "description", "aggregateRating", "review"},
		merchant: []string{
			"gtin|gtin8|gtin12|gtin13|gtin14|mpn",
			"priceValidUntil",
			"offers.shippingDetails",
			"hasMerchantReturnPolicy",
		},
	},
	"ProductGroup": {
		required:    []string{"name"},
		recommended: []string{"hasVariant", "productGroupID", "variesBy", "image", "brand"},
	},
	"Article": {
		required:    []string{"headline"},
		recommended: []string{"image", "datePublished", "dateModified", "author.name", "publisher.name"},
	},
	"NewsArticle": {
		required:    []string{"headline"},
		recommended: []string{"image", "datePublished", "dateModified", "author.name", "publisher.name"},
	},
	"BlogPosting": {
		required:    []string{"headline"},
		recommended: []string{"image", "datePublished", "dateModified", "author.name", "publisher.name"},
	},
	"Recipe": {
		required:    []string{"name"},
		recommended: []string{"image", "recipeIngredient", "recipeInstructions", "author.name", "totalTime"},
	},
	"Event": {
		required:    []string{"name", "startDate"},
		recommended: []string{"location", "image", "endDate", "eventStatus", "offers.url"},
	},
	"Organization": {
		required:    []string{"name"},
		recommended: []string{"url", "logo", "sameAs"},
	},
	"LocalBusiness": {
		required:    []string{"name"},
		recommended: []string{"address", "telephone", "openingHours|openingHoursSpecification", "geo", "priceRange"},
	},
	"Person": {
		required:    []string{"name"},
		recommended: []string{"url", "sameAs", "jobTitle"},
	},
	"BreadcrumbList": {
		required: []string{"itemListElement"},
	},
	"FAQPage": {
		required: []string{"mainEntity"},
	},
	"ItemList": {
		required: []string{"itemListElement"},
	},
	"VideoObject": {
		required:    []string{"name", "thumbnailUrl"},
		recommended: []string{"description", "uploadDate", "duration", "contentUrl|embedUrl"},
	},
	"WebSite": {
		required:    []string{"name"},
		recommended: []string{"url", "potentialAction"},
	},
}

// listProperties name the places schema.org puts thin, deliberately incomplete copies of an
// entity: a collection page's product tiles, a ProductGroup's variants, a "customers also
// bought" rail. Those copies are supposed to be minimal, so eligibility checking them would
// put a warning on every tile of every listing page and say nothing true.
var listProperties = []string{
	".itemListElement", ".hasVariant", ".isVariantOf",
	".isSimilarTo", ".isRelatedTo", ".isAccessoryOrSparePartFor",
}

// exemptFromEligibility reports whether a node's path puts it inside one of listProperties.
func exemptFromEligibility(path string) bool {
	for _, prop := range listProperties {
		if strings.Contains(path, prop) {
			return true
		}
	}
	return false
}

// requiredIssues checks every eligible node against its type's field tiers. Required gaps are
// returned as per-page issues; recommended and merchant gaps go into the rollup, which the
// caller renders once per crawl.
func requiredIssues(p *crawler.Page, g schemaorg.Graph, roll *rollup) []analyze.Issue {
	var issues []analyze.Issue
	for _, n := range g.Nodes {
		if exemptFromEligibility(n.Path) {
			continue
		}
		for _, ty := range n.Types {
			spec, known := eligibility[ty]
			if !known {
				continue
			}
			if missing := missingFields(g, n, spec.required); len(missing) > 0 {
				issues = append(issues, analyze.Issue{
					Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
					Code:    "structured-missing-required",
					Message: "Structured-data object is missing required schema.org fields",
					Data:    map[string]any{"type": ty, "missing": missing, "path": n.Path},
				})
			}
			roll.add("structured-missing-recommended", ty, missingFields(g, n, spec.recommended), p.FinalURL)
			roll.add("structured-missing-merchant", ty, missingFields(g, n, spec.merchant), p.FinalURL)
		}
	}
	return issues
}

// missingFields returns the entries of want that n does not satisfy. An entry containing "|"
// is an any-of group, satisfied by any one alternative, and is reported by its full group
// name so a reader sees the choice rather than an arbitrary member of it.
func missingFields(g schemaorg.Graph, n schemaorg.Node, want []string) []string {
	var missing []string
	for _, field := range want {
		satisfied := false
		for _, alt := range strings.Split(field, "|") {
			if g.HasValue(n, alt) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			missing = append(missing, field)
		}
	}
	return missing
}
```

- [ ] **Step 5: Thread the rollup through `Analyze`**

In `internal/analyze/structured/structured.go`, delete the Task 3 shim (`requiredIssues` and `legacyRequired` — `eligibility.go` now owns that name), and replace `Analyze` and `analyzePage`:

```go
func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	roll := newRollup()
	issues := analyze.EachPage(result, func(p *crawler.Page) []analyze.Issue {
		return a.analyzePage(p, roll)
	})
	return append(issues, roll.issues(analyze.SiteBase(result))...)
}

func (a Analyzer) analyzePage(p *crawler.Page, roll *rollup) []analyze.Issue {
```

and inside `analyzePage` change the one call site to `requiredIssues(p, g, roll)`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/analyze/structured/ -v`
Expected: PASS. `TestStructuredValidProductNoViolation` from Task 3 is now load-bearing — it only passes because the fixture carries all five required fields.

- [ ] **Step 7: Run the whole suite**

Run: `go test ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/analyze/structured/
git add internal/analyze/structured/
git commit -m "feat(structured): tier fields by rich-result requirement and roll up site-wide gaps

Required-field gaps stay per page. Recommended and Google Merchant gaps
aggregate into one info issue per type, since a theme either emits a field
or it does not and repeating that per page adds no information."
```

---

### Task 5: Integrity — duplicates, conflicts, unresolved references

**Files:**
- Create: `internal/analyze/structured/integrity.go`
- Create: `internal/analyze/structured/integrity_test.go`
- Modify: `internal/analyze/structured/structured.go`

**Interfaces:**
- Consumes: `schemaorg.Graph`, `Node.Block`, `Graph.Str`, `Graph.Resolve` from Tasks 1–2.
- Produces:
  - `func integrityIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue`
  - New codes: `structured-duplicate-type`, `structured-conflicting-value`, `structured-unresolved-id`

- [ ] **Step 1: Write the failing tests**

Create `internal/analyze/structured/integrity_test.go`:

```go
package structured_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
)

func TestDuplicateProductAcrossBlocks(t *testing.T) {
	res := page(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
	</head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-duplicate-type")
	if !ok {
		t.Fatal("expected structured-duplicate-type for two Product blocks")
	}
	if is.Data["type"] != "Product" {
		t.Errorf("expected type Product, got %v", is.Data["type"])
	}
	if is.Data["blocks"] != 2 {
		t.Errorf("expected blocks=2, got %v", is.Data["blocks"])
	}
}

func TestDuplicateWithinOneBlockIsNotFlagged(t *testing.T) {
	// Two Products in one block is a deliberate modelling choice (a comparison page, a
	// bundle), not two sources fighting over the page.
	res := page(t, `<html><head><script type="application/ld+json">
		[{"@type":"Product","name":"A"},{"@type":"Product","name":"B"}]
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-duplicate-type"); ok {
		t.Error("two Products in one block must not be flagged as duplicates")
	}
}

func TestConflictingValueBetweenBlocks(t *testing.T) {
	res := page(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99"}}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"24.99"}}</script>
	</head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-conflicting-value")
	if !ok {
		t.Fatal("expected structured-conflicting-value for disagreeing prices")
	}
	if is.Severity != analyze.Error {
		t.Errorf("expected error severity, got %q", is.Severity)
	}
	if is.Data["field"] != "offers.price" {
		t.Errorf("expected field offers.price, got %v", is.Data["field"])
	}
	values, _ := is.Data["values"].([]string)
	if len(values) != 2 {
		t.Errorf("expected both conflicting values, got %v", values)
	}
}

func TestAgreeingDuplicateHasNoConflict(t *testing.T) {
	res := page(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
	</head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-conflicting-value"); ok {
		t.Error("identical duplicates disagree about nothing")
	}
}

func TestUnresolvedIDReference(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"WebSite","name":"Shop","publisher":{"@id":"https://shop.test/#missing"}}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-unresolved-id")
	if !ok {
		t.Fatal("expected structured-unresolved-id for a dangling reference")
	}
	if is.Data["id"] != "https://shop.test/#missing" {
		t.Errorf("expected the dangling id in data, got %v", is.Data["id"])
	}
}

func TestResolvedIDReferenceIsSilent(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@graph":[{"@type":"Organization","@id":"https://shop.test/#org","name":"Shop"},
		           {"@type":"WebSite","name":"Shop","publisher":{"@id":"https://shop.test/#org"}}]}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-unresolved-id"); ok {
		t.Error("a reference whose target is on the page must not be flagged")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/structured/ -run 'Duplicate|Conflict|Agreeing|ResolvedID|UnresolvedID' -v`
Expected: FAIL — none of the three codes are emitted.

- [ ] **Step 3: Write the implementation**

Create `internal/analyze/structured/integrity.go`:

```go
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
```

- [ ] **Step 4: Wire it into `analyzePage`**

In `internal/analyze/structured/structured.go`, add one line after the `requiredIssues` call:

```go
	issues = append(issues, integrityIssues(p, g)...)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/analyze/structured/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/analyze/structured/
go vet ./internal/analyze/structured/
git add internal/analyze/structured/
git commit -m "feat(structured): flag duplicate page-level types, disagreeing values, dangling @id"
```

---

### Task 6: Integrity — value formats and on-page agreement

**Files:**
- Modify: `internal/analyze/structured/integrity.go`
- Modify: `internal/analyze/structured/integrity_test.go` (append)
- Modify: `internal/analyze/schemaorg/path.go` (add `Values`)

**Interfaces:**
- Consumes: `schemaorg.Graph`, `priceRe` from `candidates.go`.
- Produces:
  - `func (g Graph) Values(n Node, path string) []any` in `schemaorg`
  - New codes: `structured-relative-url`, `structured-invalid-date`, `structured-malformed-price`, `structured-price-mismatch`

- [ ] **Step 1: Write the failing tests**

Append to `internal/analyze/structured/integrity_test.go`:

```go
func TestRelativeURLInMarkup(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"/cdn/shop/t.jpg","url":"/products/tee"}
	</script></head><body></body></html>`)
	issues := structured.New().Analyze(context.Background(), res)
	got := findAll(issues, "structured-relative-url")
	if len(got) != 2 {
		t.Fatalf("expected a finding for each of image and url, got %d", len(got))
	}
}

func TestAbsoluteAndProtocolRelativeURLsAreFine(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"//cdn.shop.test/t.jpg","url":"https://shop.test/products/tee"}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-relative-url"); ok {
		t.Error("absolute and protocol-relative URLs are both resolvable")
	}
}

func TestInvalidDate(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"BlogPosting","headline":"Hi","datePublished":"14/09/2026"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-invalid-date")
	if !ok {
		t.Fatal("expected structured-invalid-date for a non-ISO date")
	}
	if is.Data["property"] != "datePublished" {
		t.Errorf("expected property datePublished, got %v", is.Data["property"])
	}
}

func TestISODatesAccepted(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"BlogPosting","headline":"Hi","datePublished":"2026-09-14",
		 "dateModified":"2026-09-14T08:30:00+02:00"}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-invalid-date"); ok {
		t.Error("a date-only and an RFC3339 timestamp are both valid ISO 8601")
	}
}

func TestMalformedPrice(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"$1,299.00","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-malformed-price")
	if !ok {
		t.Fatal("expected structured-malformed-price for a formatted price string")
	}
	if is.Data["value"] != "$1,299.00" {
		t.Errorf("expected the offending value in data, got %v", is.Data["value"])
	}
}

func TestNumericAndPlainStringPricesAccepted(t *testing.T) {
	for _, price := range []string{`19.99`, `"19.99"`, `"1299"`} {
		res := page(t, `<html><head><script type="application/ld+json">
			{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
			 "offers":{"@type":"Offer","price":`+price+`,"priceCurrency":"USD","availability":"https://schema.org/InStock"}}
		</script></head><body></body></html>`)
		if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-malformed-price"); ok {
			t.Errorf("price %s is well formed", price)
		}
	}
}

func TestPriceMismatchWithPage(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">$24.99</p><button>Add to cart</button></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch")
	if !ok {
		t.Fatal("expected structured-price-mismatch")
	}
	if is.Data["markup"] != "19.99" || is.Data["page"] != "24.99" {
		t.Errorf("expected markup 19.99 vs page 24.99, got %v / %v", is.Data["markup"], is.Data["page"])
	}
}

func TestPriceMatchIsSilent(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"24.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">$24.99</p><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
		t.Error("a matching price must not be flagged")
	}
}

func TestAmbiguousPageStaysSilent(t *testing.T) {
	// A sale price next to a struck-through original, or a variant selector, puts more than
	// one price on the page. There is no single on-page price to disagree with.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><s>$29.99</s><p class="price">$24.99</p><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
		t.Error("a page showing two prices is ambiguous, not wrong")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/structured/ -run 'RelativeURL|Absolute|Date|Price' -v`
Expected: FAIL — none of the four codes are emitted.

- [ ] **Step 3: Add raw-value access to `schemaorg`**

Append to `internal/analyze/schemaorg/path.go`:

```go
// Values returns every raw, undecoded value at a dotted path below n. Callers that need to
// distinguish a JSON string from a JSON number — a price written "19.99" is well-formed,
// one written "$1,299.00" is not, and both arrive as strings while a bare 19.99 does not —
// use this rather than Strs, which renders everything as a string.
func (g Graph) Values(n Node, path string) []any {
	return g.values(n.Props, strings.Split(path, "."))
}
```

- [ ] **Step 4: Write the implementation**

Append to `internal/analyze/structured/integrity.go`:

```go
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
// regexp the product candidate heuristic uses to recognize one.
func distinctPagePrices(p *crawler.Page) []float64 {
	seen := make(map[float64]bool)
	var out []float64
	for _, m := range priceRe.FindAllString(p.Doc.Find("body").Text(), -1) {
		v, ok := normalizePrice(m)
		if !ok || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// normalizePrice extracts a comparable number from either a markup price or a rendered one,
// dropping currency symbols and thousands separators.
func normalizePrice(s string) (float64, bool) {
	digits := digitsRe.FindString(s)
	if digits == "" {
		return 0, false
	}
	digits = strings.ReplaceAll(digits, ",", "")
	// A trailing separator ("19.99." at a sentence end) is not part of the number.
	digits = strings.TrimSuffix(digits, ".")
	v, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0, false
	}
	return v, true
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
```

Extend `integrity.go`'s import block to `"net/url"`, `"regexp"`, `"sort"`, `"strconv"`, `"strings"`, `"time"` plus the three internal packages. Then add one line to `integrityIssues`:

```go
	issues = append(issues, valueIssues(p, g)...)
```

Then add explanation entries for all four new codes to `internal/report/explanations.go`, using
the text given in Task 7 Step 3 for `structured-relative-url`, `structured-invalid-date`,
`structured-malformed-price` and `structured-price-mismatch`. The repo's
`TestAllAnalyzerCodesHaveExplanations` fails the whole-repo suite without them, so they belong
in this commit rather than Task 7's.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/analyze/structured/ ./internal/analyze/schemaorg/ -v`
Expected: PASS.

- [ ] **Step 6: Run the whole suite**

Run: `go test -race ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/analyze/
git add internal/analyze/
git commit -m "feat(structured): validate URL, date and price formats, and price agreement

The price-mismatch check stays silent when the page renders more than one
price, since a sale price or a variant selector makes the comparison
ambiguous rather than wrong."
```

---

### Task 7: Phase 1 explanations and documentation

**Files:**
- Modify: `internal/report/explanations.go:984-1023`
- Modify: `internal/report/explanations_test.go`
- Modify: `docs/analyzers.md:141-165`
- Modify: `CLAUDE.md`

**Interfaces:**
- Consumes: the seven new codes from Tasks 4–6.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Understand the existing contract test, then extend it**

`internal/report/explanations_test.go` (package `report`, not `report_test`) already has
`TestAllAnalyzerCodesHaveExplanations`, which statically scans every analyzer package for
issue codes and fails when one has no explanation. Read it before writing anything — most of
this task is already covered.

It has one structural blind spot that this plan walks into. The scan recognizes a code only as
a string literal in an `analyze.Issue{Code: "..."}` composite literal, or as the second
argument of an `add(analyze.Warning, "code", ...)` closure. The rollup in `rollup.go` emits
`Code: key.code` — a variable — and `eligibility.go` passes the code as the *first* argument
of `roll.add(...)`. Neither shape is visible to the scan, so `structured-missing-recommended`
and `structured-missing-merchant` would ship unexplained without failing a test.

Add an explicit guard for exactly those codes. Append to `internal/report/explanations_test.go`
(same package, so read the `explanations` map directly):

```go
// TestDynamicallyCodedIssuesHaveExplanations covers the codes TestAllAnalyzerCodesHaveExplanations
// structurally cannot see: the structured analyzer's rollup builds its Code from a variable
// rather than a string literal, so the static scan walks straight past it.
func TestDynamicallyCodedIssuesHaveExplanations(t *testing.T) {
	for _, code := range []string{"structured-missing-recommended", "structured-missing-merchant"} {
		e, ok := explanations[code]
		if !ok {
			t.Errorf("%s has no explanation", code)
			continue
		}
		if e.What == "" || e.Impact == "" || e.Fix == "" {
			t.Errorf("%s has an incomplete explanation: %+v", code, e)
		}
	}
}
```

- [ ] **Step 2: Run both tests to verify they fail**

Run: `go test ./internal/report/ -run 'TestAllAnalyzerCodes|TestDynamicallyCoded' -v`
Expected: FAIL. `TestAllAnalyzerCodesHaveExplanations` names the five literal codes from Tasks 5
and 6; `TestDynamicallyCodedIssuesHaveExplanations` names the two rollup codes.

- [ ] **Step 3: Add the explanations**

Tasks 5 and 6 have already added entries for `structured-duplicate-type`,
`structured-conflicting-value`, `structured-unresolved-id`, `structured-relative-url`,
`structured-invalid-date`, `structured-malformed-price` and `structured-price-mismatch` — the
contract test forced them into those commits. **Do not re-add any of them; a duplicate map key
will not compile.** Verify each is present and complete, then add ONLY the two codes the
contract test cannot see, because the rollup builds its `Code` from a variable:

```go
	"structured-missing-recommended": {
		What:   "Structured data of this type omits fields Google recommends for its rich result, across the pages listed.",
		Impact: "The page stays eligible for the rich result but renders a plainer one — no ratings, no author, no imagery — so it wins fewer clicks than a fully described competitor.",
		Fix:    "Add the listed properties to the template that emits this type. Because the gap repeats site-wide, one template edit fixes every affected page.",
	},
	"structured-missing-merchant": {
		What:   "Product markup omits the fields Google Shopping and free product listings read: a product identifier, price validity, shipping, and return policy.",
		Impact: "Products are ineligible for, or downranked in, Shopping and free listing surfaces, and shoppers see no shipping or returns detail before clicking.",
		Fix:    "Emit gtin (or mpn), priceValidUntil, offers.shippingDetails and hasMerchantReturnPolicy. Most of this can be templated once from store-level shipping and return settings.",
	},
```

Also update the existing `structured-missing-required` entry's `Fix` to name the new tier, and `structured-data`'s `What` — its type list now includes nested types.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/report/ -v`
Expected: PASS, including both the static contract test and the explicit rollup-code guard.

- [ ] **Step 5: Update `docs/analyzers.md`**

Replace the `structured` table (lines 147–165) with one row per code, in the order they appear in the analyzer: the three reporting codes, the three eligibility codes, the seven integrity codes, then the four candidate codes. Mark the two rollup codes' scope explicitly, since every other row is per page:

```markdown
| Code | Severity | Scope | Triggered when | `data` |
| --- | --- | --- | --- | --- |
| `structured-invalid-jsonld` | warning | page | A JSON-LD block is not valid JSON | `error` |
| `structured-none` | info | page | The page has no JSON-LD at all | — |
| `structured-data` | info | page | JSON-LD found; lists the de-duplicated `@type`s, including nested ones | `types` |
| `structured-missing-required` | warning | page | A typed object omits a field its rich result requires | `type`, `missing`, `path` |
| `structured-missing-recommended` | info | **site** | A type omits recommended fields, aggregated across the crawl | `type`, `missing`, `fields`, `pages`, `examples` |
| `structured-missing-merchant` | info | **site** | `Product` omits Google Merchant listing fields, aggregated | `type`, `missing`, `fields`, `pages`, `examples` |
| `structured-duplicate-type` | warning | page | A page-level type is declared in two or more JSON-LD blocks | `type`, `blocks` |
| `structured-conflicting-value` | error | page | Duplicate blocks disagree on `name`, `sku`, or an `offers` field | `type`, `field`, `values` |
| `structured-unresolved-id` | warning | page | An `@id` reference has no matching node on the page | `id`, `property`, `type` |
| `structured-relative-url` | warning | page | A URL property holds a relative path | `type`, `property`, `value` |
| `structured-invalid-date` | warning | page | A date property is not ISO 8601 | `type`, `property`, `value` |
| `structured-malformed-price` | warning | page | A price string carries a symbol, separator, or range | `type`, `property`, `value` |
| `structured-price-mismatch` | warning | page | `offers.price` differs from the single price rendered on the page | `markup`, `page` |
| `structured-breadcrumb-candidate` | warning | page | Breadcrumb-styled nav with ≥2 links, no `BreadcrumbList` | `links` |
| `structured-product-candidate` | warning | page | Product/price microdata, or a price plus a cart call-to-action, with no `Product`/`Offer` | `signal` |
| `structured-article-candidate` | warning | page | A 150+ word `<article>` with an author or date signal, no article type | `words` |
| `structured-video-candidate` | warning | page | A `<video>` or YouTube/Vimeo embed, no `VideoObject` | `src` |
```

Replace the two notes below the table with three:

```markdown
> **Field tiers.** Each recognized type carries a *required* set (absence blocks the rich
> result), a *recommended* set (absence degrades it), and — for `Product` — a *merchant* set
> feeding Shopping and free listings. A field written with `|` separators is an any-of group:
> `gtin|gtin8|gtin12|gtin13|gtin14|mpn` is satisfied by any one identifier.

> **Why two of them are site-scoped.** A theme either emits `aggregateRating` or it does not,
> so a recommended-field gap repeats identically on every page of a template. Those two codes
> aggregate into one issue per type, carrying the affected page count and up to five example
> URLs, instead of one issue per page.

> **Thin copies are exempt.** Objects nested under `itemListElement`, `hasVariant`,
> `isVariantOf`, `isSimilarTo`, `isRelatedTo` or `isAccessoryOrSparePartFor` are deliberately
> minimal — a collection page's product tiles carry a name and a URL and nothing else — so
> eligibility and duplicate checks skip them.

> The `*-candidate` checks are low-noise heuristics: they only fire on a fairly specific
> on-page signal and never fire when a matching `@type` is already present anywhere on the page.
```

Also update the "Source" line to list the split files:

```markdown
Source: [`internal/analyze/structured/`](../internal/analyze/structured/), reading pages
through the shared [`internal/analyze/schemaorg`](../internal/analyze/schemaorg/) parser.
```

- [ ] **Step 6: Update `CLAUDE.md`**

In the package map table, add a row after `internal/analyze`:

```markdown
| `internal/analyze/schemaorg` | Shared JSON-LD parser: flattens a page's `ld+json` into an addressable node graph (`Node`, `Graph`, dotted paths, `@id` resolution). Not an analyzer — the same role `seaurl` plays for UTM parsing. |
```

In the analyzer-seam section, extend the sentence naming `seaurl` so it reads: "`seaurl` is a shared UTM-parsing helper and `schemaorg` a shared JSON-LD parser, **not** analyzers."

- [ ] **Step 7: Verify docs match the code**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS, and `gofmt -l .` prints nothing.

Then manually confirm every code in the `docs/analyzers.md` table appears in the analyzer source:

```bash
grep -o 'structured-[a-z-]*' docs/analyzers.md | sort -u > /tmp/doc-codes
grep -rho '"structured-[a-z-]*"' internal/analyze/structured/ | tr -d '"' | sort -u > /tmp/src-codes
diff /tmp/doc-codes /tmp/src-codes
```
Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add internal/report/ docs/analyzers.md CLAUDE.md
git commit -m "docs(structured): document the eligibility tiers, integrity checks and rollup

Adds explanations for the seven new codes and a coverage test that fails
when a code ships without one."
```

---

# Phase 2 — the `shopify` analyzer

### Task 8: Shopify detection and registration

**Files:**
- Create: `internal/analyze/shopify/shopify.go`
- Create: `internal/analyze/shopify/shopify_test.go`
- Modify: `internal/runner/runner.go:87`

**Interfaces:**
- Consumes: `analyze.SiteBase` from Task 3, `crawler.Fetcher`.
- Produces:
  - `func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer`
  - `func WithProbes(on bool) Option`
  - `func pageHTML(p *crawler.Page) string`
  - `type site struct { detected bool; theme string; themeID string; signals map[string]bool }`
  - `func detect(result *crawler.Result) site`
  - New code: `shopify-detected`

- [ ] **Step 1: Write the failing test**

Create `internal/analyze/shopify/shopify_test.go`:

```go
package shopify_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/shopify"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// store builds a Result from (url, html) pairs. Body is populated as well as Doc because the
// analyzer fingerprints the raw HTML, not only the parsed DOM.
func store(t *testing.T, seed string, urlHTML map[string]string) *crawler.Result {
	t.Helper()
	res := &crawler.Result{Seed: seed}
	for u, html := range urlHTML {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("parse %s: %v", u, err)
		}
		res.Pages = append(res.Pages, &crawler.Page{
			FinalURL: u, StatusCode: 200, ContentType: "text/html",
			Body: []byte(html), Doc: doc,
		})
	}
	return res
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func findAll(issues []analyze.Issue, code string) []analyze.Issue {
	var out []analyze.Issue
	for _, is := range issues {
		if is.Code == code {
			out = append(out, is)
		}
	}
	return out
}

func run(t *testing.T, res *crawler.Result) []analyze.Issue {
	t.Helper()
	return shopify.New(nil).Analyze(context.Background(), res)
}

const shopifyHome = `<html><head>
	<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
	<script>var Shopify = Shopify || {}; Shopify.theme = {"name":"Dawn","id":123456,"theme_store_id":887,"role":"main"};</script>
</head><body><h1>Shop</h1></body></html>`

func TestDetectsShopify(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	is, ok := find(run(t, res), "shopify-detected")
	if !ok {
		t.Fatal("expected shopify-detected")
	}
	if is.URL != "https://shop.test" {
		t.Errorf("expected the site base as the issue URL, got %q", is.URL)
	}
	if is.Data["theme"] != "Dawn" {
		t.Errorf("expected theme Dawn, got %v", is.Data["theme"])
	}
	if is.Severity != analyze.Info {
		t.Errorf("expected info severity, got %q", is.Severity)
	}
}

func TestDetectsShopifyFromHeaderAlone(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": `<html><body>Hi</body></html>`})
	res.Pages[0].Header = map[string][]string{"X-Shopid": {"123456"}}
	if _, ok := find(run(t, res), "shopify-detected"); !ok {
		t.Error("expected the X-ShopId header alone to identify a Shopify store")
	}
}

func TestSilentOnNonShopifySite(t *testing.T) {
	res := store(t, "https://blog.test", map[string]string{
		"https://blog.test/":           `<html><head><meta name="generator" content="Hugo"></head><body>Hi</body></html>`,
		"https://blog.test/products/a": `<html><body><h1>A thing</h1><p>$19.99</p></body></html>`,
	})
	if issues := run(t, res); len(issues) != 0 {
		t.Errorf("expected complete silence on a non-Shopify site, got %d issues: %+v", len(issues), issues)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: FAIL — the package does not exist.

- [ ] **Step 3: Write the implementation**

Create `internal/analyze/shopify/shopify.go`:

```go
// Package shopify detects Shopify storefronts and runs Shopify-specific checks: which schema
// each page template should carry and does not, structured data injected by two competing
// sources, product markup that flattens away its variants, and the crawlable utility and
// faceted URLs Shopify generates by default.
//
// Like the wordpress analyzer it stays completely silent on a site it does not recognize, so
// enabling it costs nothing on the rest of the web.
package shopify

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Option configures the analyzer.
type Option func(*Analyzer)

// WithProbes enables the checks that fetch an extra resource (currently the /products.json
// feed). Off by default, and wired to the same --specialized flag as the WordPress security
// probes, because an audit should not make requests the crawl did not already make unless
// the operator asked for it.
func WithProbes(on bool) Option { return func(a *Analyzer) { a.probe = on } }

// Analyzer runs the Shopify-specific checks.
type Analyzer struct {
	fetcher crawler.Fetcher
	probe   bool
}

// New returns a Shopify analyzer. The fetcher is used only by the opt-in probes.
func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer {
	a := &Analyzer{fetcher: fetcher}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (Analyzer) Name() string { return "shopify" }
func (Analyzer) Description() string {
	return "Shopify detection plus store-specific checks: per-template structured-data coverage, theme/app schema conflicts, flattened product variants, and crawlable utility and faceted URLs"
}

func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	s := detect(result)
	if !s.detected {
		return nil
	}
	base := analyze.SiteBase(result)

	data := map[string]any{"signals": sortedKeys(s.signals)}
	if s.theme != "" {
		data["theme"] = s.theme
	}
	if s.themeID != "" {
		data["theme_id"] = s.themeID
	}
	issues := []analyze.Issue{{
		Analyzer: "shopify", URL: base, Severity: analyze.Info,
		Code: "shopify-detected", Message: "Site is a Shopify storefront", Data: data,
	}}
	return issues
}

// site is what detection learned about the store, aggregated across every crawled page.
type site struct {
	detected bool
	theme    string
	themeID  string
	signals  map[string]bool
}

// fingerprints are the markers that identify a Shopify storefront in page HTML. Any one is
// sufficient: they live in the shared layout, so a single crawled page is enough.
var fingerprints = []string{
	"cdn.shopify.com",
	"shopify.theme",
	"shopify-features",
	".myshopify.com",
	"/cdn/shop/",
}

// themeRe pulls the Shopify.theme object out of the inline bootstrap script the platform
// injects. The object is flat, so matching up to the first closing brace is enough.
var themeRe = regexp.MustCompile(`Shopify\.theme\s*=\s*(\{[^}]*\})`)

// detect scans every crawled HTML page for Shopify fingerprints and aggregates what it finds.
func detect(result *crawler.Result) site {
	s := site{signals: make(map[string]bool)}
	for _, p := range result.Pages {
		if p.Header.Get("X-ShopId") != "" || p.Header.Get("X-Shopify-Stage") != "" {
			s.detected = true
			s.signals["header"] = true
		}
		if !p.IsHTML() {
			continue
		}
		html := pageHTML(p)
		lower := strings.ToLower(html)
		for _, f := range fingerprints {
			if strings.Contains(lower, f) {
				s.detected = true
				s.signals[f] = true
			}
		}
		if s.theme == "" {
			s.theme, s.themeID = parseTheme(html)
		}
	}
	return s
}

// parseTheme reads the theme name and id out of the Shopify.theme bootstrap object.
func parseTheme(html string) (name, id string) {
	m := themeRe.FindStringSubmatch(html)
	if m == nil {
		return "", ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(m[1]), &obj); err != nil {
		return "", ""
	}
	name, _ = obj["name"].(string)
	switch v := obj["id"].(type) {
	case string:
		id = v
	case float64:
		id = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return name, id
}

// pageHTML returns the page's HTML source. It prefers the raw body, falling back to
// re-serializing the parsed document, so fingerprints that live in inline scripts are visible
// either way.
func pageHTML(p *crawler.Page) string {
	if len(p.Body) > 0 {
		return string(p.Body)
	}
	if p.Doc == nil {
		return ""
	}
	html, err := p.Doc.Html()
	if err != nil {
		return ""
	}
	return html
}

// sortedKeys returns a set's members in a stable order.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
```

Add `"sort"` to the import block.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: PASS, all three tests.

- [ ] **Step 5: Register the analyzer**

In `internal/runner/runner.go`, extend the CMS comment block and add the registration immediately after the `wordpress` line:

```go
	r.Register(wordpress.New(fetcher, wordpress.WithSecurityProbes(specialized)))
	r.Register(shopify.New(fetcher, shopify.WithProbes(specialized)))
```

Add the import `"github.com/Patience-dot-devl/gocrawl/internal/analyze/shopify"`.

- [ ] **Step 6: Verify the analyzer is listed**

Run: `go build -o gocrawl ./cmd/gocrawl && ./gocrawl analyzers list | grep shopify`
Expected: one line describing the `shopify` analyzer.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/analyze/shopify/ internal/runner/
go vet ./...
git add internal/analyze/shopify/ internal/runner/
git commit -m "feat(shopify): detect Shopify storefronts and register the analyzer"
```

---

### Task 9: Template classification and per-template schema gaps

> **Every Shopify fixture must carry a STRONG detection marker.** Detection was tiered during
> execution: `cdn.shopify.com`, `.myshopify.com` and `/cdn/shop/` are WEAK markers that never
> set `detected` on their own, because a non-Shopify site embedding a Shopify Buy Button emits
> exactly those. Only the `X-ShopId`/`X-Shopify-Stage` header, the `Shopify.theme` bootstrap
> object, or the `shopify-features` script make a site a store. A fixture carrying only a
> `cdn.shopify.com` script is NOT detected, the analyzer returns nil, and any test asserting
> "no finding" against it passes vacuously while any test asserting a finding fails. Every
> fixture below therefore includes the `Shopify.theme` bootstrap line.


**Files:**
- Create: `internal/analyze/shopify/template.go`
- Create: `internal/analyze/shopify/template_test.go`
- Modify: `internal/analyze/shopify/shopify.go`

**Interfaces:**
- Consumes: `schemaorg.Parse`, `Graph.HasType` from Task 1; `detect` from Task 8.
- Produces:
  - `type Template string` with constants `TemplateProduct`, `TemplateCollection`, `TemplateArticle`, `TemplateBlog`, `TemplatePage`, `TemplateHome`, `TemplateUtility`, `TemplateUnknown`
  - `func Classify(rawURL string) Template`
  - `func templateGapIssues(result *crawler.Result, base string) []analyze.Issue`
  - New code: `shopify-template-schema-gap`

- [ ] **Step 1: Write the failing test**

Create `internal/analyze/shopify/template_test.go`:

```go
package shopify_test

import (
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/shopify"
)

func TestClassify(t *testing.T) {
	cases := map[string]shopify.Template{
		"https://shop.test/":                             shopify.TemplateHome,
		"https://shop.test/products/tee":                 shopify.TemplateProduct,
		"https://shop.test/collections/all/products/tee": shopify.TemplateProduct,
		"https://shop.test/collections/all":              shopify.TemplateCollection,
		"https://shop.test/collections/all?sort_by=price": shopify.TemplateCollection,
		"https://shop.test/blogs/news/launch":            shopify.TemplateArticle,
		"https://shop.test/blogs/news":                   shopify.TemplateBlog,
		"https://shop.test/pages/about":                  shopify.TemplatePage,
		"https://shop.test/search?q=tee":                 shopify.TemplateUtility,
		"https://shop.test/cart":                         shopify.TemplateUtility,
		"https://shop.test/account/login":                shopify.TemplateUtility,
		"https://shop.test/apps/reviews":                 shopify.TemplateUnknown,
	}
	for u, want := range cases {
		if got := shopify.Classify(u); got != want {
			t.Errorf("Classify(%q) = %q, want %q", u, got, want)
		}
	}
}
```

Append to `internal/analyze/shopify/shopify_test.go`:

```go
const bareProductPage = `<html><head>
	<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
</head><body><h1>Tee</h1><p>$19.99</p><button>Add to cart</button></body></html>`

func TestTemplateSchemaGapRollsUpPerTemplate(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":            shopifyHome,
		"https://shop.test/products/a":  bareProductPage,
		"https://shop.test/products/b":  bareProductPage,
		"https://shop.test/products/c":  bareProductPage,
	})
	gaps := findAll(run(t, res), "shopify-template-schema-gap")
	var product []map[string]any
	for _, is := range gaps {
		if is.Data["template"] == "product" {
			product = append(product, is.Data)
		}
	}
	// One gap for the missing Product group, one for the missing BreadcrumbList group.
	if len(product) != 2 {
		t.Fatalf("expected 2 product-template gaps, got %d (%+v)", len(product), gaps)
	}
	for _, d := range product {
		if d["pages"] != 3 {
			t.Errorf("expected pages=3, got %v", d["pages"])
		}
		if len(d["examples"].([]string)) != 3 {
			t.Errorf("expected 3 examples, got %v", d["examples"])
		}
	}
}

func TestTemplateSchemaGapSilentWhenSchemaPresent(t *testing.T) {
	product := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
		<script type="application/ld+json">{"@type":"BreadcrumbList","itemListElement":[]}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": product})
	for _, is := range findAll(run(t, res), "shopify-template-schema-gap") {
		if is.Data["template"] == "product" {
			t.Errorf("unexpected product gap: %+v", is.Data)
		}
	}
}

func TestUtilityTemplateExpectsNothing(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":     shopifyHome,
		"https://shop.test/cart": bareProductPage,
	})
	for _, is := range findAll(run(t, res), "shopify-template-schema-gap") {
		if is.Data["template"] == "utility" {
			t.Error("utility pages should have no schema expectation")
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: FAIL — `shopify.Classify` undefined, and no `shopify-template-schema-gap` is emitted.

- [ ] **Step 3: Write the implementation**

Create `internal/analyze/shopify/template.go`:

```go
package shopify

import (
	"net/url"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Template is the kind of page a Shopify URL addresses. Shopify's URL structure is fixed by
// the platform rather than chosen per store, which is what makes classifying by path
// reliable here in a way it would not be on an arbitrary site.
type Template string

const (
	TemplateHome       Template = "home"
	TemplateProduct    Template = "product"
	TemplateCollection Template = "collection"
	TemplateArticle    Template = "article"
	TemplateBlog       Template = "blog"
	TemplatePage       Template = "page"
	TemplateUtility    Template = "utility"
	TemplateUnknown    Template = "unknown"
)

// Classify returns the Shopify template a URL addresses.
func Classify(rawURL string) Template {
	u, err := url.Parse(rawURL)
	if err != nil {
		return TemplateUnknown
	}
	segs := pathSegments(u.Path)
	if len(segs) == 0 {
		return TemplateHome
	}
	switch segs[0] {
	case "products":
		if len(segs) >= 2 {
			return TemplateProduct
		}
	case "collections":
		// /collections/<handle>/products/<handle> is the same product reached through a
		// collection; Shopify serves it from the product template.
		if len(segs) >= 4 && segs[2] == "products" {
			return TemplateProduct
		}
		if len(segs) >= 2 {
			return TemplateCollection
		}
	case "blogs":
		if len(segs) >= 3 {
			return TemplateArticle
		}
		if len(segs) == 2 {
			return TemplateBlog
		}
	case "pages":
		if len(segs) >= 2 {
			return TemplatePage
		}
	case "search", "cart", "account", "challenge", "checkouts", "orders":
		return TemplateUtility
	}
	return TemplateUnknown
}

// pathSegments splits a URL path into its non-empty segments.
func pathSegments(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// expectation is one schema.org requirement for a template, satisfied by any one of anyOf.
// The alternatives exist because more than one type legitimately answers the same need — a
// collection listing is equally well described by CollectionPage or ItemList.
type expectation struct {
	template Template
	anyOf    []string
	label    string
}

// expectations is the schema each Shopify template should carry. Utility templates are absent
// deliberately: a cart or an account page has nothing to say to a search engine.
var expectations = []expectation{
	{TemplateHome, []string{"Organization", "LocalBusiness"}, "Organization"},
	{TemplateHome, []string{"WebSite"}, "WebSite"},
	{TemplateProduct, []string{"Product", "ProductGroup"}, "Product"},
	{TemplateProduct, []string{"BreadcrumbList"}, "BreadcrumbList"},
	{TemplateCollection, []string{"CollectionPage", "ItemList"}, "CollectionPage or ItemList"},
	{TemplateCollection, []string{"BreadcrumbList"}, "BreadcrumbList"},
	{TemplateArticle, []string{"BlogPosting", "Article", "NewsArticle"}, "BlogPosting"},
	{TemplateArticle, []string{"BreadcrumbList"}, "BreadcrumbList"},
	{TemplateBlog, []string{"Blog", "CollectionPage"}, "Blog"},
	{TemplatePage, []string{"WebPage", "AboutPage", "ContactPage", "FAQPage"}, "WebPage"},
}

// gapKey identifies one accumulated template gap.
type gapKey struct {
	template Template
	label    string
}

// gapEntry counts the pages of a template missing one expectation.
type gapEntry struct {
	anyOf    []string
	pages    int
	examples []string
}

// maxExamples caps the example URLs on an aggregated finding, matching the structured
// analyzer's rollup.
const maxExamples = 5

// templateGapIssues reports, per template, the schema every page of that template is missing.
// Shopify templates are shared across every page they render, so a gap is a property of the
// template — reporting it per page would repeat one fact several hundred times.
func templateGapIssues(result *crawler.Result, base string) []analyze.Issue {
	entries := make(map[gapKey]*gapEntry)
	var order []gapKey

	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		tmpl := Classify(p.FinalURL)
		g, _ := schemaorg.Parse(p.Doc)
		for _, exp := range expectations {
			if exp.template != tmpl || g.HasType(exp.anyOf...) {
				continue
			}
			key := gapKey{template: tmpl, label: exp.label}
			e, ok := entries[key]
			if !ok {
				e = &gapEntry{anyOf: exp.anyOf}
				entries[key] = e
				order = append(order, key)
			}
			e.pages++
			if len(e.examples) < maxExamples {
				e.examples = append(e.examples, p.FinalURL)
			}
		}
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i].template != order[j].template {
			return order[i].template < order[j].template
		}
		return order[i].label < order[j].label
	})

	var issues []analyze.Issue
	for _, key := range order {
		e := entries[key]
		issues = append(issues, analyze.Issue{
			Analyzer: "shopify", URL: base, Severity: analyze.Warning,
			Code:    "shopify-template-schema-gap",
			Message: "Shopify " + string(key.template) + " pages have no " + key.label + " structured data",
			Data: map[string]any{
				"template": string(key.template),
				"expected": e.anyOf,
				"pages":    e.pages,
				"examples": e.examples,
			},
		})
	}
	return issues
}
```

- [ ] **Step 4: Call it from `Analyze`**

In `internal/analyze/shopify/shopify.go`, after building the `shopify-detected` issue:

```go
	issues = append(issues, templateGapIssues(result, base)...)
	return issues
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/analyze/shopify/
go vet ./internal/analyze/shopify/
git add internal/analyze/shopify/
git commit -m "feat(shopify): classify pages by template and report per-template schema gaps"
```

---

### Task 10: App attribution, schema conflicts, client-side injection

**Files:**
- Create: `internal/analyze/shopify/schema.go`
- Create: `internal/analyze/shopify/schema_test.go`
- Modify: `internal/analyze/shopify/shopify.go`

**Interfaces:**
- Consumes: `schemaorg.Parse`, `Node.Block`, `Graph.OfType` from Task 1; `Classify` from Task 9.
- Produces:
  - `func blockSources(doc *goquery.Document) []string`
  - `func schemaIssues(p *crawler.Page, g schemaorg.Graph, tmpl Template) []analyze.Issue`
  - New codes: `shopify-schema-app-conflict`, `shopify-schema-client-injected`

- [ ] **Step 1: Write the failing test**

Create `internal/analyze/shopify/schema_test.go`:

```go
package shopify_test

import (
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
)

func TestSchemaAppConflict(t *testing.T) {
	// The theme emits its own Product, then an SEO app injects a second one.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99"}}</script>
		<script src="https://cdn.shopify.com/extensions/abc/json-ld-for-seo/assets/app.js"></script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"24.99"}}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-schema-app-conflict")
	if !ok {
		t.Fatal("expected shopify-schema-app-conflict")
	}
	if is.Severity != analyze.Error {
		t.Errorf("expected error severity, got %q", is.Severity)
	}
	sources, _ := is.Data["sources"].([]string)
	if len(sources) != 2 {
		t.Fatalf("expected two attributed sources, got %v", sources)
	}
}

func TestTwoThemeBlocksAreNotAConflict(t *testing.T) {
	// A theme emitting Product and BreadcrumbList in two blocks is normal, and two Product
	// blocks from one source is a duplicate (the structured analyzer's finding), not an app
	// conflict.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	if _, ok := find(run(t, res), "shopify-schema-app-conflict"); ok {
		t.Error("two blocks attributed to the same source are not an app conflict")
	}
}

func TestAppAttributedByBlockIDAttribute(t *testing.T) {
	// Several apps mark their own block rather than loading a recognizable script first.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
		<script type="application/ld+json" id="schemaplus-product">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-schema-app-conflict")
	if !ok {
		t.Fatal("expected a conflict attributed via the block's own id")
	}
	sources, _ := is.Data["sources"].([]string)
	var sawApp bool
	for _, s := range sources {
		if s == "Schema Plus" {
			sawApp = true
		}
	}
	if !sawApp {
		t.Errorf("expected Schema Plus among the sources, got %v", sources)
	}
}

func TestClientInjectedSchemaWarning(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script src="https://cdn.shopify.com/extensions/abc/searchpie/assets/app.js"></script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-schema-client-injected")
	if !ok {
		t.Fatal("expected shopify-schema-client-injected")
	}
	if is.Data["app"] != "SearchPie" {
		t.Errorf("expected the app named in data, got %v", is.Data["app"])
	}
}

func TestNoClientInjectionWarningWhenSchemaIsPresent(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script src="https://cdn.shopify.com/extensions/abc/searchpie/assets/app.js"></script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	if _, ok := find(run(t, res), "shopify-schema-client-injected"); ok {
		t.Error("no warning is needed when the raw HTML already carries JSON-LD")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/shopify/ -run Schema -v`
Expected: FAIL — neither code is emitted.

- [ ] **Step 3: Write the implementation**

Create `internal/analyze/shopify/schema.go`:

```go
package shopify

import (
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// themeSource is the attribution given to a JSON-LD block with no recognizable app before it.
// A Shopify theme renders its structured data inline from Liquid, so it leaves no marker of
// its own — "not an app" is the only signal available, and it is the right default.
const themeSource = "theme"

// schemaApps maps a marker found in a script's URL or attributes to the app that owns it.
// Attribution reads attributes only, never a script's contents: an app's name routinely
// appears inside unrelated inline JSON, and matching on that would misattribute blocks.
var schemaApps = []struct{ marker, name string }{
	{"json-ld-for-seo", "JSON-LD for SEO"},
	{"jsonld-for-seo", "JSON-LD for SEO"},
	{"schemaplus", "Schema Plus"},
	{"schema-plus", "Schema Plus"},
	{"searchpie", "SearchPie"},
	{"schemaapp", "Schema App"},
	{"seoant", "SEOAnt"},
	{"yoast", "Yoast for Shopify"},
	{"tinyimg", "TinyIMG"},
	{"smart-seo", "Smart SEO"},
	{"avada-seo", "Avada SEO"},
}

// appFor returns the app owning a marker string, or "" when none matches.
func appFor(s string) string {
	lower := strings.ToLower(s)
	for _, app := range schemaApps {
		if strings.Contains(lower, app.marker) {
			return app.name
		}
	}
	return ""
}

// scriptMarkers returns the attribute text worth fingerprinting on a script element.
func scriptMarkers(s *goquery.Selection) string {
	src, _ := s.Attr("src")
	id, _ := s.Attr("id")
	class, _ := s.Attr("class")
	return src + " " + id + " " + class
}

// blockSources attributes each JSON-LD block on the page to the app that emitted it, indexed
// the same way schemaorg.Node.Block is: by position among the page's ld+json scripts in
// document order. A block is attributed by its own attributes first, then by the nearest
// recognized app script preceding it, and falls back to the theme.
func blockSources(doc *goquery.Document) []string {
	var sources []string
	current := themeSource
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		typ, _ := s.Attr("type")
		marker := scriptMarkers(s)
		if typ != "application/ld+json" {
			if app := appFor(marker); app != "" {
				current = app
			}
			return
		}
		if own := appFor(marker); own != "" {
			sources = append(sources, own)
			return
		}
		sources = append(sources, current)
	})
	return sources
}

// pageApps returns every schema app whose script appears on the page.
func pageApps(doc *goquery.Document) []string {
	seen := make(map[string]bool)
	var out []string
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		app := appFor(scriptMarkers(s))
		if app == "" || seen[app] {
			return
		}
		seen[app] = true
		out = append(out, app)
	})
	return out
}

// schemaIssues reports structured data emitted by two competing sources, and pages where an
// SEO app is installed but the raw HTML carries no JSON-LD at all.
func schemaIssues(p *crawler.Page, g schemaorg.Graph, tmpl Template) []analyze.Issue {
	var issues []analyze.Issue
	sources := blockSources(p.Doc)

	// Attribute each block that declares a page-level Product, then look for disagreement
	// about who owns it. Two blocks from one source are a duplicate, which the structured
	// analyzer already reports; two *sources* is the Shopify-specific failure, because
	// neither the theme nor the app knows the other exists.
	seen := make(map[string]bool)
	var attributed []string
	for _, n := range g.OfType("Product") {
		if n.Block >= len(sources) {
			continue
		}
		src := sources[n.Block]
		if seen[src] {
			continue
		}
		seen[src] = true
		attributed = append(attributed, src)
	}
	if len(attributed) > 1 {
		sort.Strings(attributed)
		issues = append(issues, analyze.Issue{
			Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Error,
			Code:    "shopify-schema-app-conflict",
			Message: "Product structured data is emitted by more than one source on this page",
			Data:    map[string]any{"sources": attributed},
		})
	}

	// A store that pays for an SEO app and shows no JSON-LD in the raw HTML is almost
	// certainly having it injected client-side, which a raw crawl cannot see. Say so rather
	// than reporting the page as bare.
	if len(g.Nodes) == 0 && tmpl != TemplateUtility && tmpl != TemplateUnknown {
		if apps := pageApps(p.Doc); len(apps) > 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Info,
				Code:    "shopify-schema-client-injected",
				Message: "A structured-data app is installed but the raw HTML carries no JSON-LD",
				Data:    map[string]any{"app": apps[0], "apps": apps, "template": string(tmpl)},
			})
		}
	}
	return issues
}
```

- [ ] **Step 4: Add the per-page pass to `Analyze`**

In `internal/analyze/shopify/shopify.go`, replace the tail of `Analyze` with:

```go
	issues = append(issues, templateGapIssues(result, base)...)
	issues = append(issues, analyze.EachPage(result, a.analyzePage)...)
	return issues
}

// analyzePage runs the checks that are genuinely per page — a conflict or a flattened variant
// is a property of one template render, not of the store.
func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	g, _ := schemaorg.Parse(p.Doc)
	tmpl := Classify(p.FinalURL)
	return schemaIssues(p, g, tmpl)
}
```

Add the `schemaorg` import.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/analyze/shopify/
go vet ./internal/analyze/shopify/
git add internal/analyze/shopify/
git commit -m "feat(shopify): attribute JSON-LD blocks to theme or app and flag conflicts"
```

---

### Task 11: Variant modelling

**Files:**
- Modify: `internal/analyze/shopify/schema.go`
- Modify: `internal/analyze/shopify/schema_test.go` (append)

**Interfaces:**
- Consumes: `schemaorg.Graph`, `Graph.HasValue`, `Graph.OfType`, `Classify`.
- Produces:
  - `func variantIssues(p *crawler.Page, g schemaorg.Graph, tmpl Template) []analyze.Issue`
  - New codes: `shopify-flat-variant-product`, `shopify-single-offer-range`

- [ ] **Step 1: Write the failing test**

Append to `internal/analyze/shopify/schema_test.go`:

```go
const variantSelector = `<variant-radios><select name="id">
	<option value="1">Small</option><option value="2">Medium</option></select></variant-radios>`

func TestFlatVariantProduct(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-flat-variant-product")
	if !ok {
		t.Fatal("expected shopify-flat-variant-product")
	}
	if is.Data["variants"] != 2 {
		t.Errorf("expected variants=2, got %v", is.Data["variants"])
	}
}

func TestProductGroupSuppressesFlatVariantWarning(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"ProductGroup","name":"Tee","productGroupID":"T",
		 "hasVariant":[{"@type":"Product","name":"Tee S"},{"@type":"Product","name":"Tee M"}]}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	if _, ok := find(run(t, res), "shopify-flat-variant-product"); ok {
		t.Error("a ProductGroup already models the variants")
	}
}

func TestSingleVariantProductIsNotFlagged(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1><select name="id"><option value="1">Default</option></select></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	if _, ok := find(run(t, res), "shopify-flat-variant-product"); ok {
		t.Error("a product with one variant has nothing to model")
	}
}

func TestSingleOfferForMultiplePrices(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `
		<script type="application/json" id="ProductJson-product-template">
			{"variants":[{"id":1,"price":1999},{"id":2,"price":2499}]}
		</script></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-single-offer-range")
	if !ok {
		t.Fatal("expected shopify-single-offer-range")
	}
	if is.Data["prices"] != 2 {
		t.Errorf("expected prices=2, got %v", is.Data["prices"])
	}
}

func TestUniformVariantPricesNeedNoRange(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `
		<script type="application/json" id="ProductJson-product-template">
			{"variants":[{"id":1,"price":1999},{"id":2,"price":1999}]}
		</script></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	if _, ok := find(run(t, res), "shopify-single-offer-range"); ok {
		t.Error("variants that all cost the same are correctly described by one Offer")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/shopify/ -run 'Variant|Offer' -v`
Expected: FAIL — neither code is emitted.

- [ ] **Step 3: Write the implementation**

Append to `internal/analyze/shopify/schema.go`:

```go
// variantIssues reports product pages whose markup describes fewer things than the page sells.
// Shopify's own variant machinery is visible in the DOM, so the gap between what the page
// offers and what the markup says is directly measurable.
func variantIssues(p *crawler.Page, g schemaorg.Graph, tmpl Template) []analyze.Issue {
	if tmpl != TemplateProduct {
		return nil
	}
	variants := variantCount(p.Doc)
	if variants < 2 {
		return nil
	}
	var issues []analyze.Issue

	products := g.OfType("Product")
	modelled := g.HasType("ProductGroup")
	for _, n := range products {
		if g.HasValue(n, "hasVariant") || g.HasValue(n, "isVariantOf") {
			modelled = true
		}
	}
	if len(products) > 0 && !modelled {
		issues = append(issues, analyze.Issue{
			Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Warning,
			Code:    "shopify-flat-variant-product",
			Message: "Product markup describes one item but the page sells several variants",
			Data:    map[string]any{"variants": variants},
		})
	}

	// A single Offer states one price. When the variants do not share a price, that price is
	// wrong for most of them, and an AggregateOffer with a low/high range is the honest shape.
	prices := variantPrices(p.Doc)
	if len(prices) > 1 && !g.HasType("AggregateOffer") {
		singleOffer := false
		for _, n := range products {
			if len(g.NodesAt(n, "offers")) == 1 {
				singleOffer = true
			}
		}
		if singleOffer {
			issues = append(issues, analyze.Issue{
				Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Info,
				Code:    "shopify-single-offer-range",
				Message: "Variants are priced differently but the markup states a single Offer price",
				Data:    map[string]any{"prices": len(prices), "variants": variants},
			})
		}
	}
	return issues
}

// variantSelectors are the DOM shapes Shopify themes use to let a shopper pick a variant.
var variantSelectors = []string{
	`select[name="id"] option`,
	`input[name="id"]`,
	`variant-radios input`,
	`variant-selects option`,
	`[data-variant-id]`,
}

// variantCount returns the largest number of variants any selector on the page exposes.
func variantCount(doc *goquery.Document) int {
	most := 0
	for _, sel := range variantSelectors {
		if n := doc.Find(sel).Length(); n > most {
			most = n
		}
	}
	return most
}

// variantPrices returns the distinct variant prices from the product JSON Shopify themes
// embed for their own JavaScript. The units do not matter — themes emit cents here and
// decimals elsewhere — because only the count of distinct values is used.
func variantPrices(doc *goquery.Document) []float64 {
	var raw string
	doc.Find(`script[type="application/json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		id, _ := s.Attr("id")
		if !strings.HasPrefix(id, "ProductJson") && !strings.Contains(id, "product-json") {
			return true
		}
		raw = s.Text()
		return false
	})
	if raw == "" {
		return nil
	}
	var payload struct {
		Variants []struct {
			Price any `json:"price"`
		} `json:"variants"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	seen := make(map[float64]bool)
	var out []float64
	for _, v := range payload.Variants {
		var price float64
		switch t := v.Price.(type) {
		case float64:
			price = t
		case string:
			parsed, err := strconv.ParseFloat(strings.ReplaceAll(t, ",", ""), 64)
			if err != nil {
				continue
			}
			price = parsed
		default:
			continue
		}
		if seen[price] {
			continue
		}
		seen[price] = true
		out = append(out, price)
	}
	return out
}
```

Add `"encoding/json"` and `"strconv"` to `schema.go`'s imports.

- [ ] **Step 4: Call it from `analyzePage`**

```go
	issues := schemaIssues(p, g, tmpl)
	issues = append(issues, variantIssues(p, g, tmpl)...)
	return issues
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: PASS.

- [ ] **Step 6: Run the whole suite**

Run: `go test -race ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -w internal/analyze/shopify/
git add internal/analyze/shopify/
git commit -m "feat(shopify): flag product markup that flattens away variants and price ranges"
```

---

### Task 12: Phase 2 explanations and documentation

**Files:**
- Modify: `internal/report/explanations.go`
- Modify: `docs/analyzers.md`

**Interfaces:**
- Consumes: the six codes from Tasks 8–11.

- [ ] **Step 1: Confirm the existing contract test covers these codes**

Every code added in Tasks 8–11 is emitted as a string literal inside an `analyze.Issue{...}`
composite literal, which is exactly the shape `TestAllAnalyzerCodesHaveExplanations` in
`internal/report/explanations_test.go` scans for. No new test is needed — the existing one
already fails for all six.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/report/ -run TestAllAnalyzerCodes -v`
Expected: FAIL, naming `shopify-detected`, `shopify-template-schema-gap`,
`shopify-schema-app-conflict`, `shopify-schema-client-injected`,
`shopify-flat-variant-product` and `shopify-single-offer-range`.

- [ ] **Step 3: Verify the explanations (do NOT re-add them)**

Tasks 8-11 have already added these six entries — the contract test forced each into the commit
that introduced its code. **Re-adding any of them is a duplicate map key and will not compile.**
Verify each is present with a non-empty What/Impact/Fix and accurately describes the code as
built, correcting wording only where the implementation diverged from what the entry claims.
For reference, the intended text is:

```go
	"shopify-detected": {
		What:   "The site was identified as a Shopify storefront, with the theme it runs.",
		Impact: "Informational. It tells the rest of this report which template each URL renders and which checks apply.",
		Fix:    "No action needed.",
	},
	"shopify-template-schema-gap": {
		What:   "Every crawled page of this Shopify template is missing a schema.org type that template should carry.",
		Impact: "The whole template is ineligible for its rich result — product pages without Product markup win no price, rating, or availability treatment in search.",
		Fix:    "Add the markup to the template once (theme Liquid or an SEO app), and every page it renders gains it. Check the example URLs to confirm the template was identified correctly.",
	},
	"shopify-schema-app-conflict": {
		What:   "Two sources — typically the theme and an SEO app — each emit Product structured data on the same page, without knowing about each other.",
		Impact: "Search engines pick one and discard the other, so the page may be represented by stale or incomplete markup, and any disagreement between them risks a structured-data manual action.",
		Fix:    "Pick one owner. Either disable structured data in the theme (most themes expose a setting) or turn off the app's Product schema, so a single source emits it.",
	},
	"shopify-schema-client-injected": {
		What:   "A structured-data app is installed, but the HTML served to the crawler contains no JSON-LD — the app is injecting it with JavaScript.",
		Impact: "Google renders JavaScript and will usually see it, but rendering is deferred and other crawlers and AI answer engines often do not render at all, so the markup is invisible to them.",
		Fix:    "Re-run the crawl with --render headless to confirm what the app emits. Prefer server-rendered structured data in theme Liquid, where every crawler sees it on the first fetch.",
	},
	"shopify-flat-variant-product": {
		What:   "The page sells several variants but its markup describes a single Product, with no ProductGroup or hasVariant.",
		Impact: "Search engines see one item where the store sells several, so variant-level attributes — size, colour, per-variant price and availability — never reach Shopping or rich results.",
		Fix:    "Emit a ProductGroup with productGroupID and variesBy, and one Product per variant under hasVariant.",
	},
	"shopify-single-offer-range": {
		What:   "The product's variants are priced differently, but the markup states one Offer price.",
		Impact: "The stated price is wrong for every variant that does not match it, and a price that contradicts the page risks suppression of price-bearing rich results.",
		Fix:    "Use an AggregateOffer with lowPrice and highPrice, or give each variant its own Offer under a ProductGroup.",
	},
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/report/ -v`
Expected: PASS.

- [ ] **Step 5: Add the `shopify` section to `docs/analyzers.md`**

Insert immediately after the `wordpress` section, and add `shopify` to the analyzer list in the document's intro paragraph (line 11, which currently reads "the CMS-specific `wordpress`"):

```markdown
---

## `shopify` — Shopify detection and store-specific checks (CMS)

Source: [`internal/analyze/shopify/`](../internal/analyze/shopify/).
Fingerprints Shopify from the crawled HTML (`cdn.shopify.com` and `/cdn/shop/` asset paths,
the `Shopify.theme` bootstrap object, the `shopify-features` script, a `.myshopify.com`
reference) or from the `X-ShopId` / `X-Shopify-Stage` response headers; on a non-Shopify site
it stays completely silent.

Because Shopify's URL structure is fixed by the platform rather than chosen per store, every
crawled URL is classified into a **template** — `home`, `product`, `collection`, `article`,
`blog`, `page`, `utility` — and checked against the schema that template should carry. A gap
is a property of the template, not of one page, so it is reported once per template with the
affected page count and up to five example URLs.

| Code | Severity | Scope | Triggered when | `data` |
| --- | --- | --- | --- | --- |
| `shopify-detected` | info | site | The site is identified as Shopify | `theme`, `theme_id`, `signals` |
| `shopify-template-schema-gap` | warning | site | Every page of a template lacks the schema it should carry | `template`, `expected`, `pages`, `examples` |
| `shopify-schema-app-conflict` | error | page | `Product` JSON-LD is emitted by two different attributed sources | `sources` |
| `shopify-schema-client-injected` | info | page | A schema app is installed but the raw HTML has no JSON-LD | `app`, `apps`, `template` |
| `shopify-flat-variant-product` | warning | page | 2+ variants on the page, one flat `Product`, no `ProductGroup`/`hasVariant` | `variants` |
| `shopify-single-offer-range` | info | page | Variants priced differently, a single `Offer` and no `AggregateOffer` | `prices`, `variants` |

**Expected schema per template:**

| Template | Path shape | Expected |
| --- | --- | --- |
| `home` | `/` | `Organization` (or `LocalBusiness`), `WebSite` |
| `product` | `/products/<h>`, `/collections/<c>/products/<h>` | `Product` (or `ProductGroup`), `BreadcrumbList` |
| `collection` | `/collections/<h>` | `CollectionPage` or `ItemList`, `BreadcrumbList` |
| `article` | `/blogs/<b>/<a>` | `BlogPosting` (or `Article`/`NewsArticle`), `BreadcrumbList` |
| `blog` | `/blogs/<b>` | `Blog` or `CollectionPage` |
| `page` | `/pages/<h>` | `WebPage`, `AboutPage`, `ContactPage` or `FAQPage` |
| `utility` | `/cart`, `/search`, `/account/*`, `/challenge` | nothing |

> **Attribution is by script attributes, never contents.** Each JSON-LD block is attributed to
> the app named in its own `src`/`id`/`class`, else to the nearest recognized app script before
> it, else to the theme. An app's name appears inside unrelated inline JSON often enough that
> matching on script *contents* would misattribute blocks. Recognized apps: JSON-LD for SEO,
> Schema Plus, SearchPie, Schema App, SEOAnt, Yoast for Shopify, TinyIMG, Smart SEO, Avada SEO.

> **Raw mode sees what every crawler sees.** Shopify themes render JSON-LD server-side, so a
> raw crawl finds it. Several SEO apps inject it with JavaScript instead; when one is installed
> and the raw HTML has none, `shopify-schema-client-injected` says so rather than reporting the
> page as bare. Re-run with `--render headless` to see what the app emits.
```

- [ ] **Step 6: Verify docs match the code**

```bash
grep -o 'shopify-[a-z-]*' docs/analyzers.md | sort -u > /tmp/doc-codes
grep -rho '"shopify-[a-z-]*"' internal/analyze/shopify/ | tr -d '"' | sort -u > /tmp/src-codes
diff /tmp/doc-codes /tmp/src-codes
```
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/report/ docs/analyzers.md
git commit -m "docs(shopify): document template classification and the structured-data checks"
```

---

# Phase 3 — general Shopify SEO checks

Phase 3 is the least related to the structured-data goal and the safest to defer. It is
independently shippable: nothing in Phases 1–2 depends on it.

### Task 13: Crawlable utility and faceted URLs

**Files:**
- Create: `internal/analyze/shopify/seo.go`
- Create: `internal/analyze/shopify/seo_test.go`
- Modify: `internal/analyze/shopify/shopify.go`

**Interfaces:**
- Consumes: `Classify` from Task 9.
- Produces:
  - `func indexable(p *crawler.Page) bool`
  - `func canonicalOf(doc *goquery.Document) string`
  - `func seoIssues(p *crawler.Page, tmpl Template) []analyze.Issue`
  - New codes: `shopify-indexable-utility`, `shopify-indexable-facet`

- [ ] **Step 1: Write the failing test**

Create `internal/analyze/shopify/seo_test.go`:

```go
package shopify_test

import "testing"

const shopifyShell = `<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>`

func TestIndexableUtilityPage(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":            shopifyHome,
		"https://shop.test/search?q=tee": `<html><head>` + shopifyShell + `</head><body>Results</body></html>`,
	})
	is, ok := find(run(t, res), "shopify-indexable-utility")
	if !ok {
		t.Fatal("expected shopify-indexable-utility for a crawlable /search page")
	}
	if is.URL != "https://shop.test/search?q=tee" {
		t.Errorf("expected the finding on the utility URL, got %q", is.URL)
	}
}

func TestNoindexedUtilityPageIsFine(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/": shopifyHome,
		"https://shop.test/search?q=tee": `<html><head>` + shopifyShell +
			`<meta name="robots" content="noindex,follow"></head><body>Results</body></html>`,
	})
	if _, ok := find(run(t, res), "shopify-indexable-utility"); ok {
		t.Error("a noindexed utility page is already handled")
	}
}

func TestUtilityNoindexViaHeader(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":     shopifyHome,
		"https://shop.test/cart": `<html><head>` + shopifyShell + `</head><body>Cart</body></html>`,
	})
	for _, p := range res.Pages {
		if p.FinalURL == "https://shop.test/cart" {
			p.Header = map[string][]string{"X-Robots-Tag": {"noindex"}}
		}
	}
	if _, ok := find(run(t, res), "shopify-indexable-utility"); ok {
		t.Error("X-Robots-Tag: noindex must count the same as a meta robots tag")
	}
}

func TestIndexableFacetedCollection(t *testing.T) {
	faceted := `<html><head>` + shopifyShell +
		`<link rel="canonical" href="https://shop.test/collections/all?sort_by=price-asc"></head><body>Grid</body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":                              shopifyHome,
		"https://shop.test/collections/all?sort_by=price-asc": faceted,
	})
	is, ok := find(run(t, res), "shopify-indexable-facet")
	if !ok {
		t.Fatal("expected shopify-indexable-facet for a self-canonical sorted collection")
	}
	if is.Data["parameter"] != "sort_by" {
		t.Errorf("expected parameter sort_by, got %v", is.Data["parameter"])
	}
}

func TestFacetCanonicalisedToUnfilteredCollectionIsFine(t *testing.T) {
	faceted := `<html><head>` + shopifyShell +
		`<link rel="canonical" href="https://shop.test/collections/all"></head><body>Grid</body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":                              shopifyHome,
		"https://shop.test/collections/all?sort_by=price-asc": faceted,
	})
	if _, ok := find(run(t, res), "shopify-indexable-facet"); ok {
		t.Error("a facet canonicalised to the unfiltered collection is correctly handled")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/shopify/ -run 'Utility|Facet' -v`
Expected: FAIL — neither code is emitted.

- [ ] **Step 3: Write the implementation**

Create `internal/analyze/shopify/seo.go`:

```go
package shopify

import (
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// facetParams are the query parameters Shopify's own collection filtering and sorting use.
// Each one multiplies a single collection into an unbounded set of near-identical URLs.
var facetParams = []string{"sort_by", "filter.", "constraint", "pf_", "grid_list"}

// indexable reports whether a page is open to indexing — no noindex in either the meta robots
// tag or the X-Robots-Tag header.
func indexable(p *crawler.Page) bool {
	if strings.Contains(strings.ToLower(p.Header.Get("X-Robots-Tag")), "noindex") {
		return false
	}
	if p.Doc == nil {
		return true
	}
	robots, _ := p.Doc.Find(`meta[name="robots"]`).First().Attr("content")
	return !strings.Contains(strings.ToLower(robots), "noindex")
}

// canonicalOf returns the page's declared canonical URL, or "" when it has none.
func canonicalOf(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	href, _ := doc.Find(`link[rel="canonical"]`).First().Attr("href")
	return strings.TrimSpace(href)
}

// seoIssues reports the crawlable URLs Shopify generates by default that a store rarely wants
// in an index.
func seoIssues(p *crawler.Page, tmpl Template) []analyze.Issue {
	var issues []analyze.Issue

	// Utility pages carry no content worth ranking, and /search in particular generates an
	// unbounded set of URLs from whatever anyone links to.
	if tmpl == TemplateUtility && indexable(p) {
		issues = append(issues, analyze.Issue{
			Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Warning,
			Code:    "shopify-indexable-utility",
			Message: "A Shopify utility page is crawlable and open to indexing",
			Data:    map[string]any{"path": pathOf(p.FinalURL)},
		})
	}

	// A sorted or filtered collection is the same set of products in a different order. Left
	// self-canonical, each permutation competes with the collection it came from.
	if tmpl == TemplateCollection {
		if param, ok := facetParam(p.FinalURL); ok {
			canonical := canonicalOf(p.Doc)
			if canonical == "" || sameURL(canonical, p.FinalURL) {
				issues = append(issues, analyze.Issue{
					Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Warning,
					Code:    "shopify-indexable-facet",
					Message: "A sorted or filtered collection URL is indexable and not canonicalised to the unfiltered collection",
					Data:    map[string]any{"parameter": param, "canonical": canonical},
				})
			}
		}
	}
	return issues
}

// facetParam returns the first faceting parameter present in a URL's query.
func facetParam(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	for key := range u.Query() {
		for _, f := range facetParams {
			if key == f || strings.HasPrefix(key, f) {
				return f, true
			}
		}
	}
	return "", false
}

// sameURL compares two URLs ignoring a trailing slash and fragment, which differ without
// meaning anything.
func sameURL(a, b string) bool {
	return strings.TrimRight(stripFragment(a), "/") == strings.TrimRight(stripFragment(b), "/")
}

// stripFragment drops a URL's fragment.
func stripFragment(raw string) string {
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		return raw[:i]
	}
	return raw
}

// pathOf returns a URL's path for use in a finding's data.
func pathOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Path
}
```

- [ ] **Step 4: Call it from `analyzePage`**

```go
	issues = append(issues, seoIssues(p, tmpl)...)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/analyze/shopify/
go vet ./internal/analyze/shopify/
git add internal/analyze/shopify/
git commit -m "feat(shopify): flag indexable utility pages and self-canonical faceted collections"
```

---

### Task 14: Duplicate product paths

**Files:**
- Modify: `internal/analyze/shopify/seo.go`
- Modify: `internal/analyze/shopify/seo_test.go` (append)

**Interfaces:**
- Consumes: `canonicalOf`, `sameURL`, `pathSegments` from Tasks 9 and 13.
- Produces: new code `shopify-duplicate-product-path`

- [ ] **Step 1: Write the failing test**

Append to `internal/analyze/shopify/seo_test.go`:

```go
func TestDuplicateProductPathPreservesLocale(t *testing.T) {
	// A Markets storefront serves the product under its locale prefix. The canonical we
	// recommend must keep that prefix; /products/tee does not exist in the en-ca market.
	nested := `<html><head>` + shopifyShell + `</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":                                    shopifyHome,
		"https://shop.test/en-ca/collections/all/products/tee":  nested,
	})
	is, ok := find(run(t, res), "shopify-duplicate-product-path")
	if !ok {
		t.Fatal("expected shopify-duplicate-product-path on a locale-prefixed nested product URL")
	}
	if is.Data["canonical_should_be"] != "https://shop.test/en-ca/products/tee" {
		t.Errorf("canonical must keep the locale prefix, got %v", is.Data["canonical_should_be"])
	}
}

func TestDuplicateProductPath(t *testing.T) {
	nested := `<html><head>` + shopifyShell + `</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":                              shopifyHome,
		"https://shop.test/collections/all/products/tee": nested,
	})
	is, ok := find(run(t, res), "shopify-duplicate-product-path")
	if !ok {
		t.Fatal("expected shopify-duplicate-product-path for an uncanonicalised nested product URL")
	}
	if is.Data["canonical_should_be"] != "https://shop.test/products/tee" {
		t.Errorf("expected the canonical target in data, got %v", is.Data["canonical_should_be"])
	}
}

func TestNestedProductWithCorrectCanonicalIsFine(t *testing.T) {
	nested := `<html><head>` + shopifyShell +
		`<link rel="canonical" href="https://shop.test/products/tee"></head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":                              shopifyHome,
		"https://shop.test/collections/all/products/tee": nested,
	})
	if _, ok := find(run(t, res), "shopify-duplicate-product-path"); ok {
		t.Error("a nested product URL canonicalised to /products/<handle> is correct")
	}
}

func TestCanonicalProductPathIsNotFlagged(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":             shopifyHome,
		"https://shop.test/products/tee": `<html><head>` + shopifyShell + `</head><body><h1>Tee</h1></body></html>`,
	})
	if _, ok := find(run(t, res), "shopify-duplicate-product-path"); ok {
		t.Error("the canonical product path is not a duplicate of itself")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/shopify/ -run DuplicateProductPath -v`
Expected: FAIL — the code is not emitted.

- [ ] **Step 3: Write the implementation**

Append to `seoIssues` in `internal/analyze/shopify/seo.go`, before the final `return`:

```go
	// Shopify serves every product at /products/<handle> and again under each collection it
	// belongs to. The nested copies are the same page; without a canonical pointing at the
	// short path, a product with ten collections is ten competing URLs.
	if want, ok := canonicalProductURL(p.FinalURL); ok {
		if canonical := canonicalOf(p.Doc); canonical == "" || !sameURL(canonical, want) {
			issues = append(issues, analyze.Issue{
				Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Warning,
				Code:    "shopify-duplicate-product-path",
				Message: "A product is served under a collection path without a canonical pointing at /products/<handle>",
				Data:    map[string]any{"canonical": canonical, "canonical_should_be": want},
			})
		}
	}
```

And add:

```go
// canonicalProductURL returns the /products/<handle> form of a nested
// /collections/<c>/products/<handle> URL. It returns false for any other URL, including the
// canonical product path itself.
//
// A Shopify Markets storefront serves every route under a locale prefix, so the match skips
// that prefix via localeOffset — but the returned URL PUTS IT BACK. Recommending a canonical
// that drops the locale would point the store at a path that does not exist in that market.
func canonicalProductURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	segs := pathSegments(u.Path)
	off := localeOffset(segs)
	rest := segs[off:]
	if len(rest) < 4 || rest[0] != "collections" || rest[2] != "products" {
		return "", false
	}
	prefix := ""
	if off > 0 {
		prefix = "/" + segs[0]
	}
	return u.Scheme + "://" + u.Host + prefix + "/products/" + rest[3], true
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/analyze/shopify/
git add internal/analyze/shopify/
git commit -m "feat(shopify): flag collection-nested product URLs without a canonical"
```

---

### Task 15: The opt-in `/products.json` probe

**Files:**
- Modify: `internal/analyze/shopify/shopify.go`
- Create: `internal/analyze/shopify/probe_test.go`

**Interfaces:**
- Consumes: `Analyzer.fetcher`, `Analyzer.probe` from Task 8.
- Produces:
  - `func (a Analyzer) productsJSONProbe(ctx context.Context, base string) []analyze.Issue`
  - New code: `shopify-products-json-exposed`

- [ ] **Step 1: Write the failing test**

Create `internal/analyze/shopify/probe_test.go`:

```go
package shopify_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/shopify"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// stubFetcher answers a fixed set of URLs and records which were requested, so a test can
// assert that the probe stays off unless it was asked for.
type stubFetcher struct {
	pages     map[string]*crawler.Page
	requested []string
}

func (f *stubFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Page, error) {
	f.requested = append(f.requested, rawURL)
	if p, ok := f.pages[rawURL]; ok {
		return p, nil
	}
	return &crawler.Page{RequestedURL: rawURL, FinalURL: rawURL, StatusCode: 404}, nil
}

func TestProductsJSONProbeOffByDefault(t *testing.T) {
	f := &stubFetcher{pages: map[string]*crawler.Page{}}
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	shopify.New(f).Analyze(context.Background(), res)
	if len(f.requested) != 0 {
		t.Errorf("the analyzer must make no requests unless probes are enabled, got %v", f.requested)
	}
}

func TestProductsJSONProbeReportsAnExposedFeed(t *testing.T) {
	body := `{"products":[{"id":1,"title":"Tee","handle":"tee","variants":[{"id":11,"price":"19.99"}]}]}`
	f := &stubFetcher{pages: map[string]*crawler.Page{
		"https://shop.test/products.json?limit=1": {
			FinalURL: "https://shop.test/products.json?limit=1", StatusCode: 200,
			ContentType: "application/json", Body: []byte(body),
		},
	}}
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	issues := shopify.New(f, shopify.WithProbes(true)).Analyze(context.Background(), res)
	is, ok := find(issues, "shopify-products-json-exposed")
	if !ok {
		t.Fatal("expected shopify-products-json-exposed")
	}
	if is.URL != "https://shop.test/products.json" {
		t.Errorf("expected the feed URL without the probe's limit, got %q", is.URL)
	}
}

func TestProductsJSONProbeSilentWhenClosed(t *testing.T) {
	f := &stubFetcher{pages: map[string]*crawler.Page{}} // every URL 404s
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	issues := shopify.New(f, shopify.WithProbes(true)).Analyze(context.Background(), res)
	if _, ok := find(issues, "shopify-products-json-exposed"); ok {
		t.Error("a 404 feed is not exposed")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/analyze/shopify/ -run ProductsJSON -v`
Expected: FAIL — the code is not emitted. `TestProductsJSONProbeOffByDefault` passes already, which is correct: it is a guard, not a driver.

- [ ] **Step 3: Write the implementation**

Append to `internal/analyze/shopify/shopify.go`:

```go
// productsJSONProbe checks whether the store's /products.json feed answers unauthenticated
// requests. Shopify serves it by default, and it returns the full catalogue — titles,
// handles, variant prices, inventory-adjacent detail — in a form competitors and scrapers can
// consume wholesale. It is opt-in because it is the analyzer's only extra request; the probe
// asks for a single product, which is enough to confirm the endpoint is open.
func (a Analyzer) productsJSONProbe(ctx context.Context, base string) []analyze.Issue {
	if !a.probe || a.fetcher == nil {
		return nil
	}
	feed := base + "/products.json"
	page, err := a.fetcher.Fetch(ctx, feed+"?limit=1")
	if err != nil || page == nil || page.StatusCode != 200 {
		return nil
	}
	var payload struct {
		Products []struct {
			Handle string `json:"handle"`
		} `json:"products"`
	}
	if err := json.Unmarshal(page.Body, &payload); err != nil {
		return nil
	}
	if len(payload.Products) == 0 {
		return nil
	}
	return []analyze.Issue{{
		Analyzer: "shopify", URL: feed, Severity: analyze.Info,
		Code:    "shopify-products-json-exposed",
		Message: "The store's /products.json catalogue feed answers unauthenticated requests",
		Data:    map[string]any{"sample_handle": payload.Products[0].Handle},
	}}
}
```

Call it from `Analyze`, after the per-page pass:

```go
	issues = append(issues, a.productsJSONProbe(ctx, base)...)
	return issues
```

The `ctx` parameter of `Analyze` is currently discarded — change its signature back to naming it (`func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue`) if Task 8 left it as `_`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/analyze/shopify/ -v`
Expected: PASS.

- [ ] **Step 5: Confirm the flag is wired end to end**

```bash
go build -o gocrawl ./cmd/gocrawl
grep -n 'shopify.WithProbes' internal/runner/runner.go
```
Expected: the registration passes `specialized`, so `--specialized` enables the probe.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/analyze/shopify/
go vet ./...
git add internal/analyze/shopify/
git commit -m "feat(shopify): add the opt-in /products.json catalogue-exposure probe"
```

---

### Task 16: Phase 3 documentation and final verification

**Files:**
- Modify: `internal/report/explanations.go`
- Modify: `docs/analyzers.md`
- Modify: `CLAUDE.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: the four codes from Tasks 13–15.

- [ ] **Step 1: Run the contract test to see the gap**

The four codes from Tasks 13–15 are all string literals in `analyze.Issue{...}` composite
literals, so `TestAllAnalyzerCodesHaveExplanations` covers them with no change.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/report/ -run TestAllAnalyzerCodes -v`
Expected: FAIL, naming `shopify-indexable-utility`, `shopify-indexable-facet`,
`shopify-duplicate-product-path` and `shopify-products-json-exposed`.

- [ ] **Step 3: Verify the explanations (do NOT re-add them)**

Tasks 13-15 have already added these four entries — the contract test forced each into the
commit that introduced its code. **Re-adding any of them is a duplicate map key and will not
compile.** Verify each is present, complete and accurate against the code as built. For
reference, the intended text is:

```go
	"shopify-indexable-utility": {
		What:   "A Shopify utility page — /search, /cart, /account or /challenge — is crawlable and not marked noindex.",
		Impact: "These pages carry no content worth ranking, and /search in particular generates an unbounded set of URLs from whatever anyone links to, wasting crawl budget and risking thin-content pages in the index.",
		Fix:    "Add <meta name=\"robots\" content=\"noindex,follow\"> to the utility templates in theme.liquid, or disallow the paths in robots.txt.liquid.",
	},
	"shopify-indexable-facet": {
		What:   "A sorted or filtered collection URL (?sort_by=, ?filter.*=) is indexable and does not canonicalise to the unfiltered collection.",
		Impact: "Each permutation is the same products in a different order, competing with the collection it came from and multiplying crawl budget across near-identical pages.",
		Fix:    "Emit a canonical pointing at the unfiltered collection URL on every faceted variant, and consider disallowing the parameters in robots.txt.",
	},
	"shopify-duplicate-product-path": {
		What:   "A product is served at /collections/<collection>/products/<handle> without a canonical pointing at /products/<handle>.",
		Impact: "Shopify serves a product once per collection it belongs to, so a product in ten collections becomes ten competing URLs, splitting link signals across all of them.",
		Fix:    "Most themes already emit the right canonical; if yours does not, set it to {{ product.url }} prefixed with the shop URL rather than {{ canonical_url }} in a collection context.",
	},
	"shopify-products-json-exposed": {
		What:   "The store's /products.json endpoint answers unauthenticated requests with the product catalogue.",
		Impact: "Titles, handles, variants and prices can be scraped wholesale by competitors and repricing bots. Shopify enables this by default, so it is worth a deliberate decision rather than an accident.",
		Fix:    "If the catalogue is not meant to be public, block /products.json (and /collections/*/products.json) at the CDN or in robots.txt. Note that robots.txt deters crawlers but does not prevent access.",
	},
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/report/ -v`
Expected: PASS.

- [ ] **Step 5: Extend the `shopify` docs section**

Add the four rows to the code table in `docs/analyzers.md`:

```markdown
| `shopify-indexable-utility` | warning | page | `/search`, `/cart`, `/account/*` or `/challenge` is indexable | `path` |
| `shopify-indexable-facet` | warning | page | A `?sort_by=` / `?filter.*=` collection URL is indexable and not canonicalised to the unfiltered collection | `parameter`, `canonical` |
| `shopify-duplicate-product-path` | warning | page | `/collections/<c>/products/<h>` without a canonical to `/products/<h>` | `canonical`, `canonical_should_be` |
| `shopify-products-json-exposed` | info | site | `/products.json` returns a catalogue feed (**opt-in**, `--specialized`) | `sample_handle` |
```

And add a note below the tables:

```markdown
> **One check is opt-in.** `shopify-products-json-exposed` is the only check here that fetches
> anything the crawl did not already fetch, so it rides `--specialized` alongside the WordPress
> security probes. Everything else is passive.
```

- [ ] **Step 6: Update `CLAUDE.md`**

In the registered-analyzer list, change "the CMS-specific `wordpress`" to "the CMS-specific
`wordpress` and `shopify`". In the package map, add:

```markdown
| `internal/analyze/shopify` | Shopify detection, URL-template classification, per-template structured-data coverage, theme/app schema conflicts, variant modelling, and Shopify-specific crawl-hygiene checks. |
```

In the "Opt-in analyzer modes" section, extend the `Specialized` bullet to mention the
Shopify probe: "`wordpress` security probes, the Shopify `/products.json` probe, `aeo`
answer-lead, `geo` quotable-density."

- [ ] **Step 7: Fix the stale row in the design spec**

`docs/superpowers/specs/2026-09-14-structured-data-shopify-design.md` carries an older codes
table whose `structured-missing-required` row still lists `` `type`, `missing` (unchanged) ``.
The code gained a `path` key during Phase 1, so that row is stale. Add `path` to it. The spec is
the authority this plan argues from, so a knowingly-stale row there has a cost even though the
live reference doc (`docs/analyzers.md`) is correct.

- [ ] **Step 9: Update `README.md`**

Find the analyzer list in `README.md` and add `shopify` beside `wordpress`. If the README
carries an example report or feature bullets mentioning structured data, add a line naming
rich-result eligibility and Shopify template coverage.

- [ ] **Step 8: Full verification**

```bash
gofmt -l .
go vet ./...
go test -race ./...
golangci-lint run
go build -o gocrawl ./cmd/gocrawl
./gocrawl analyzers list
```
Expected: `gofmt -l .` prints nothing; vet, tests and lint all pass; `analyzers list` shows
`shopify` after `wordpress`.

Then confirm every emitted code is documented and explained:

```bash
grep -rhoE '"(structured|shopify)-[a-z-]+"' internal/analyze/ | tr -d '"' | sort -u > /tmp/emitted
grep -oE '(structured|shopify)-[a-z-]+' docs/analyzers.md | sort -u > /tmp/documented
grep -oE '"(structured|shopify)-[a-z-]+":' internal/report/explanations.go | tr -d '":' | sort -u > /tmp/explained
diff /tmp/emitted /tmp/documented && diff /tmp/emitted /tmp/explained
```
Expected: no output from either diff.

- [ ] **Step 10: End-to-end smoke test against a real store**

```bash
./gocrawl crawl https://kith.com --depth 1 --max-pages 25 --analyzers shopify,structured --output /tmp/kith.json
jq -r '.issues[] | select(.analyzer=="shopify" or .analyzer=="structured") | "\(.severity)\t\(.code)\t\(.url)"' /tmp/kith.json | sort | uniq -c | sort -rn
```
Expected: `shopify-detected` present; `structured-missing-merchant` present for `Product`;
no `structured-price-mismatch` storm (if one appears on most pages, the mismatch heuristic
needs its ambiguity guard reviewed before this ships).

Also check `structured-unresolved-id` specifically. Task 5's implementer flagged that
`Graph.Resolve` is scoped to one page, so a site that declares `Organization` or `WebSite` once
(on the homepage) and references it by `@id` from every other page would raise this warning
site-wide. If the smoke test shows it firing on most pages, the fix is a site-wide `@id` index
built in `Analyze` before the per-page pass — report only references unresolvable anywhere in
the crawl. Do not make that change speculatively; make it only if the evidence appears.

- [ ] **Step 11: Commit**

```bash
git add internal/report/ docs/ CLAUDE.md README.md
git commit -m "docs(shopify): document the crawl-hygiene checks and the opt-in feed probe"
```

---

## Notes for the executor

- **Phases are shippable boundaries.** Phase 1 (Tasks 1–7) is a complete, useful change on its
  own; so is Phase 2 (Tasks 8–12). If the work is interrupted, stop at a phase boundary.
- **Three behaviour changes are intended**, all in Phase 1: nested types now appear in
  `structured-data` and suppress candidates, a bare top-level `Offer` is no longer
  required-field checked, and `Product`'s required tier grows from one field to five. Each has
  a named test. Any *other* change to existing output is a bug — find it before continuing.
- **The field tables in `eligibility.go` are data, not logic.** If a finding looks wrong in
  practice, the fix is almost always moving a field between tiers, not changing the checks.
- **The `structured-price-mismatch` guard matters.** It is the one check here that can produce
  a site-wide false-positive storm. If Task 16's smoke test shows it firing on most pages,
  widen the ambiguity guard before shipping.
