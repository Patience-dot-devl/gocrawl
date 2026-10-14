package redirectcheck

// Verdict is the overall pass/fail assessment of one row.
type Verdict string

const (
	VerdictOK              Verdict = "ok"
	VerdictWarning         Verdict = "warning"
	VerdictError           Verdict = "error"
	VerdictSkippedExternal Verdict = "skipped-external"
	VerdictSkippedDynamic  Verdict = "skipped-dynamic"
)

// RowResult is what checking one rule against the live site produced. The JSON names are what
// the web API returns for each row.
type RowResult struct {
	Scope               Scope    `json:"scope"`
	SourceStatus        int      `json:"source_status"`
	SourceFinalURL      string   `json:"source_final_url"`
	SourceMatchesTarget bool     `json:"source_matches_target"`
	TargetStatus        int      `json:"target_status"`
	OriginalInSitemap   bool     `json:"original_in_sitemap"`
	TargetInSitemap     bool     `json:"target_in_sitemap"`
	Verdict             Verdict  `json:"verdict"`
	Notes               []string `json:"notes,omitempty"`
}

var verdictRank = map[Verdict]int{
	"":             0,
	VerdictOK:      0,
	VerdictWarning: 1,
	VerdictError:   2,
}

// escalate returns whichever of current/next is the more severe verdict, so a row's Verdict
// always reflects the worst finding triggered while its Notes list every finding.
func escalate(current, next Verdict) Verdict {
	if verdictRank[next] > verdictRank[current] {
		return next
	}
	return current
}
