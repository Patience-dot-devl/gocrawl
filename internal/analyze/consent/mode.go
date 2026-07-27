package consent

import (
	"regexp"
	"sort"
	"strings"
)

// Google Consent Mode v2 requires four signals. The first two shipped with v1; ad_user_data
// and ad_personalization were added in March 2024 and are mandatory for advertisers serving
// the EEA — without them, Google Ads stops collecting for those users entirely.
var (
	v1Signals = []string{"ad_storage", "analytics_storage"}
	v2Signals = []string{"ad_user_data", "ad_personalization"}
)

// reConsentCall matches a Consent Mode call in either of the two forms sites use: the gtag
// shim, gtag('consent', 'default', {...}), and the raw dataLayer push it compiles down to,
// dataLayer.push(['consent', 'default', {...}]). The object body is matched without nested
// braces, which is safe — consent objects are flat maps of signal to 'granted'/'denied'.
var reConsentCall = regexp.MustCompile(`(?is)['"]consent['"]\s*,\s*['"](default|update)['"]\s*,\s*\{([^{}]*)\}`)

// reSignal pulls one signal:state pair out of a consent object body.
var reSignal = regexp.MustCompile(`(?i)['"]?([a-z_]+)['"]?\s*:\s*['"](granted|denied)['"]`)

// reWaitForUpdate matches the wait_for_update directive, which holds tags back for a number
// of milliseconds so an asynchronously-loaded CMP can deliver the real choice first.
var reWaitForUpdate = regexp.MustCompile(`(?i)['"]?wait_for_update['"]?\s*:\s*(\d+)`)

// consentMode is what a page's Consent Mode configuration amounts to.
type consentMode struct {
	// found is true when any consent default/update call was seen.
	found bool
	// hasDefault is true when at least one call was a 'default' — the one that matters, since
	// it is what applies before the visitor chooses.
	hasDefault bool
	// defaults maps each signal named in a default call to "granted" or "denied".
	defaults map[string]string
	// waitForUpdate is the declared wait in milliseconds, 0 when absent.
	waitForUpdate int
	// regional is true when a default call is scoped with a `region` key. A site may then
	// grant by default globally and deny only in the EEA, which is legitimate, so the
	// granted-by-default check stands down.
	regional bool
}

// parseConsentMode extracts the Consent Mode configuration from a page's combined script
// text. blob should preserve original case: signal names are lowercase by convention but
// region codes are not.
func parseConsentMode(blob string) consentMode {
	m := consentMode{defaults: map[string]string{}}
	for _, call := range reConsentCall.FindAllStringSubmatch(blob, -1) {
		m.found = true
		command, body := strings.ToLower(call[1]), call[2]
		if command != "default" {
			continue
		}
		m.hasDefault = true
		if strings.Contains(strings.ToLower(body), "region") {
			m.regional = true
		}
		if w := reWaitForUpdate.FindStringSubmatch(body); w != nil {
			m.waitForUpdate = atoi(w[1])
		}
		for _, sig := range reSignal.FindAllStringSubmatch(body, -1) {
			name := strings.ToLower(sig[1])
			// wait_for_update and region are directives, not consent signals.
			if name == "wait_for_update" || name == "region" {
				continue
			}
			// A signal granted in one regional block and denied in another is, overall, granted
			// somewhere — record the permissive value so the check below sees it.
			if m.defaults[name] != "granted" {
				m.defaults[name] = strings.ToLower(sig[2])
			}
		}
	}
	return m
}

// missingV2Signals returns the Consent Mode v2 signals a default call never mentions.
func (m consentMode) missingV2Signals() []string {
	var missing []string
	for _, s := range v2Signals {
		if _, ok := m.defaults[s]; !ok {
			missing = append(missing, s)
		}
	}
	return missing
}

// grantedByDefault returns the tracking signals a default call grants up front, sorted.
// Granting ad_storage or analytics_storage before the visitor has answered defeats the point
// of Consent Mode; functionality_storage and security_storage are not consent-gated and are
// excluded.
func (m consentMode) grantedByDefault() []string {
	gated := append(append([]string{}, v1Signals...), v2Signals...)
	var granted []string
	for _, s := range gated {
		if m.defaults[s] == "granted" {
			granted = append(granted, s)
		}
	}
	sort.Strings(granted)
	return granted
}

// declaresV1 reports whether the configuration names at least one of the original v1 signals
// — the marker of a real Consent Mode setup as opposed to an incidental match.
func (m consentMode) declaresV1() bool {
	for _, s := range v1Signals {
		if _, ok := m.defaults[s]; ok {
			return true
		}
	}
	return false
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}
