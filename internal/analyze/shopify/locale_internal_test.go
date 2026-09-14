package shopify

import "testing"

// TestLocaleOffsetExcludesKnownRoots is the negative for the roots exclusion in localeOffset:
// a real Shopify route must never be mistaken for a Markets locale prefix and skipped, even
// though none of today's roots happen to be two letters (see the localeRootSegments comment).
func TestLocaleOffsetExcludesKnownRoots(t *testing.T) {
	for root := range localeRootSegments {
		if got := localeOffset([]string{root, "x"}); got != 0 {
			t.Errorf("localeOffset([%q, \"x\"]) = %d, want 0: a known Shopify route must not be treated as a locale prefix", root, got)
		}
	}
}
