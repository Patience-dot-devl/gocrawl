// Package redirectcheck verifies a redirect-rule CSV export (HubSpot's URL Redirects tool
// schema) against a live site: whether each rule's source still redirects correctly, whether
// its target is a live page, and whether both sides agree with the site's current sitemap.xml.
package redirectcheck

import (
	"encoding/csv"
	"fmt"
	"io"
)

// expectedHeader is the exact HubSpot URL Redirects export column order.
var expectedHeader = []string{
	"Original URL", "Redirect to", "Redirect type", "Redirect style", "Priority",
	"Match query strings", "Ignore trailing slash", "Ignore protocol", "Disable if page exists", "Note",
}

// Rule is one row of the redirect-rule CSV. Columns are kept as raw strings so they can be
// echoed back verbatim in the output report.
type Rule struct {
	Original            string
	Target              string
	RedirectType        string
	RedirectStyle       string
	Priority            string
	MatchQueryStrings   string
	IgnoreTrailingSlash string
	IgnoreProtocol      string
	DisableIfPageExists string
	Note                string
}

// ParseCSV reads a HubSpot-format redirect-rule export. It returns an error if the header
// doesn't match the expected column schema.
func ParseCSV(r io.Reader) ([]Rule, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	if !equalStrings(header, expectedHeader) {
		return nil, fmt.Errorf("unexpected CSV columns\n got:  %v\n want: %v", header, expectedHeader)
	}

	var rules []Rule
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV row: %w", err)
		}
		rules = append(rules, Rule{
			Original:            record[0],
			Target:              record[1],
			RedirectType:        record[2],
			RedirectStyle:       record[3],
			Priority:            record[4],
			MatchQueryStrings:   record[5],
			IgnoreTrailingSlash: record[6],
			IgnoreProtocol:      record[7],
			DisableIfPageExists: record[8],
			Note:                record[9],
		})
	}
	return rules, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
