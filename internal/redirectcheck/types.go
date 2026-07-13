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

// RowResult is what checking one rule against the live site produced.
type RowResult struct {
	Scope               Scope
	SourceStatus        int
	SourceFinalURL      string
	SourceMatchesTarget bool
	TargetStatus        int
	OriginalInSitemap   bool
	TargetInSitemap     bool
	Verdict             Verdict
	Notes               []string
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
