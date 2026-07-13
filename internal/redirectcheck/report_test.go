package redirectcheck_test

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestWriteCSVIncludesAllRowsInOrder(t *testing.T) {
	rules := []redirectcheck.Rule{
		{Original: "/old", Target: "/new"},
		{Original: "/gone", Target: "https://other-site.com/x"},
	}
	results := []redirectcheck.RowResult{
		{Scope: redirectcheck.ScopeInScope, SourceStatus: 301, SourceFinalURL: "https://example.com/new", SourceMatchesTarget: true, TargetStatus: 200, TargetInSitemap: true, Verdict: redirectcheck.VerdictOK},
		{Scope: redirectcheck.ScopeExternal, Verdict: redirectcheck.VerdictSkippedExternal},
	}
	var buf bytes.Buffer
	if err := redirectcheck.WriteCSV(&buf, rules, results); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("reading back CSV: %v", err)
	}
	if len(rows) != 3 { // header + 2 data rows
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0][len(rows[0])-1] != "notes" {
		t.Errorf("last header column = %q, want notes", rows[0][len(rows[0])-1])
	}
	if rows[1][len(rows[1])-2] != "ok" {
		t.Errorf("row 1 verdict = %q, want ok", rows[1][len(rows[1])-2])
	}
	if rows[2][len(rows[2])-2] != "skipped-external" {
		t.Errorf("row 2 verdict = %q, want skipped-external", rows[2][len(rows[2])-2])
	}
	if rows[2][10] != "external" { // first appended column ("scope")
		t.Errorf("row 2 scope = %q, want external", rows[2][10])
	}
}
