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
		if arr, ok := v.([]any); ok {
			return arr
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

// Values returns every raw, undecoded value at a dotted path below n. Callers that need to
// distinguish a JSON string from a JSON number — a price written "19.99" is well-formed,
// one written "$1,299.00" is not, and both arrive as strings while a bare 19.99 does not —
// use this rather than Strs, which renders everything as a string.
func (g Graph) Values(n Node, path string) []any {
	return g.values(n.Props, strings.Split(path, "."))
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
