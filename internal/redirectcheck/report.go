package redirectcheck

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

// WriteCSV writes rules and their check results as a CSV: the original columns verbatim, plus
// the appended verdict columns. Every input row is present in the output, in order.
func WriteCSV(w io.Writer, rules []Rule, results []RowResult) error {
	cw := csv.NewWriter(w)
	header := append(append([]string{}, expectedHeader...),
		"scope", "source_status", "source_final_url", "source_matches_target",
		"target_status", "original_in_sitemap", "target_in_sitemap", "verdict", "notes",
	)
	if err := cw.Write(header); err != nil {
		return err
	}
	for i, rule := range rules {
		res := results[i]
		record := []string{
			rule.Original, rule.Target, rule.RedirectType, rule.RedirectStyle, rule.Priority,
			rule.MatchQueryStrings, rule.IgnoreTrailingSlash, rule.IgnoreProtocol, rule.DisableIfPageExists, rule.Note,
			string(res.Scope),
			statusStr(res.SourceStatus), res.SourceFinalURL, boolStr(res.SourceMatchesTarget),
			statusStr(res.TargetStatus), boolStr(res.OriginalInSitemap), boolStr(res.TargetInSitemap),
			string(res.Verdict), strings.Join(res.Notes, "; "),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func statusStr(code int) string {
	if code == 0 {
		return ""
	}
	return strconv.Itoa(code)
}

func boolStr(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}
