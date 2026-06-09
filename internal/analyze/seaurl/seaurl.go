// Package seaurl parses UTM campaign parameters from URLs. It is shared by the SEA
// analyzers (utm, landing) so UTM tagging is interpreted consistently and tested once.
// It depends only on the standard library — analyzers cannot reach the crawler's
// unexported URL helpers, so all parsing here goes through net/url.
package seaurl

import (
	"net/url"
	"sort"
	"strings"
)

// CanonicalUTMKeys are the five standard UTM parameters in canonical (lowercase) form.
var CanonicalUTMKeys = []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"}

// RequiredUTMKeys are the three keys Google requires for manual campaign tagging. A link
// carrying some but not all of these is "partially tagged".
var RequiredUTMKeys = []string{"utm_source", "utm_medium", "utm_campaign"}

var canonicalSet = map[string]bool{
	"utm_source": true, "utm_medium": true, "utm_campaign": true,
	"utm_term": true, "utm_content": true,
}

// UTM holds the parsed UTM parameters of a single URL.
type UTM struct {
	Values      map[string]string // canonical key -> first non-empty value (trimmed)
	Duplicates  []string          // canonical keys that appeared more than once (sorted)
	CasingMixed []string          // raw keys that were not already lowercase, e.g. "UTM_Source" (sorted)
	Empty       []string          // canonical keys present only with empty/whitespace values (sorted)
	Raw         url.Values        // the full parsed query, for callers that need more
}

// Tagged reports whether any UTM parameter is present (with a value or empty).
func (u UTM) Tagged() bool { return len(u.Values) > 0 || len(u.Empty) > 0 }

// PresentKeys returns the canonical UTM keys that appear in the URL (with any value,
// including empty), in canonical order.
func (u UTM) PresentKeys() []string {
	present := make(map[string]bool, len(u.Values)+len(u.Empty))
	for k := range u.Values {
		present[k] = true
	}
	for _, k := range u.Empty {
		present[k] = true
	}
	var out []string
	for _, k := range CanonicalUTMKeys {
		if present[k] {
			out = append(out, k)
		}
	}
	return out
}

// Missing returns the required keys absent from this URL, in the order given.
func (u UTM) Missing(required []string) []string {
	present := make(map[string]bool, len(u.Values)+len(u.Empty))
	for k := range u.Values {
		present[k] = true
	}
	for _, k := range u.Empty {
		present[k] = true
	}
	var out []string
	for _, k := range required {
		if !present[k] {
			out = append(out, k)
		}
	}
	return out
}

// Parse extracts UTM parameters from a raw URL. It is tolerant of parse errors, returning
// an empty (untagged) UTM rather than failing.
func Parse(rawURL string) UTM {
	u, err := url.Parse(rawURL)
	if err != nil {
		return UTM{Values: map[string]string{}}
	}
	return ParseQuery(u.Query())
}

// ParseQuery is the core parsing logic, exposed for callers that already hold a url.Values.
// Keys are matched case-insensitively and canonicalized to lowercase; the original casing is
// recorded in CasingMixed. Outputs are sorted so results are deterministic across the
// randomized map iteration order.
func ParseQuery(q url.Values) UTM {
	out := UTM{Values: map[string]string{}, Raw: q}
	seen := map[string]bool{} // canonical keys already encountered (detects cross-casing dupes)
	dupSet := map[string]bool{}
	emptySet := map[string]bool{}
	var casing []string

	for rawKey, vals := range q {
		lk := strings.ToLower(rawKey)
		if !canonicalSet[lk] {
			continue
		}
		if rawKey != lk {
			casing = append(casing, rawKey)
		}
		if len(vals) > 1 { // the same raw key carried multiple values
			dupSet[lk] = true
		}
		if seen[lk] { // the same canonical key under a different spelling
			dupSet[lk] = true
		}
		seen[lk] = true

		v := ""
		if len(vals) > 0 {
			v = strings.TrimSpace(vals[0])
		}
		if v == "" {
			emptySet[lk] = true
			continue
		}
		if _, ok := out.Values[lk]; ok {
			dupSet[lk] = true // already set via another casing
		} else {
			out.Values[lk] = v
		}
	}

	// A key with at least one non-empty value isn't "empty".
	for k := range out.Values {
		delete(emptySet, k)
	}

	out.Duplicates = sortedKeys(dupSet)
	out.Empty = sortedKeys(emptySet)
	sort.Strings(casing)
	out.CasingMixed = casing
	return out
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
