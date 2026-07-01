package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
)

// Reporter serializes a Diff to a writer.
type Reporter interface {
	Write(w io.Writer, d *Diff) error
}

// For returns the Reporter for the given format ("text" or "json"; default text).
func For(format string) Reporter {
	switch strings.ToLower(format) {
	case "json":
		return JSONReporter{}
	default:
		return TextReporter{}
	}
}

// JSONReporter writes an indented JSON diff.
type JSONReporter struct{}

// Write implements Reporter.
func (JSONReporter) Write(w io.Writer, d *Diff) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

// TextReporter writes a human-readable diff for the terminal.
type TextReporter struct {
	// MaxList caps how many issues/URLs are listed per section (0 = the default of 20).
	MaxList int
}

// Write implements Reporter.
func (t TextReporter) Write(w io.Writer, d *Diff) error {
	max := t.MaxList
	if max == 0 {
		max = 20
	}
	var b strings.Builder

	fmt.Fprintf(&b, "Comparing crawls of %s\n", d.Current.Seed)
	fmt.Fprintf(&b, "  base:    %s  (%d pages)\n", orDash(d.Base.FinishedAt), d.Base.PagesCrawled)
	fmt.Fprintf(&b, "  current: %s  (%d pages)\n\n", orDash(d.Current.FinishedAt), d.Current.PagesCrawled)

	if d.Unchanged() {
		b.WriteString("No change: same issues and same pages crawled.\n")
		_, err := io.WriteString(w, b.String())
		return err
	}

	fmt.Fprintf(&b, "Issues: %s new, %s resolved, %d unchanged\n",
		sevSummary(d.Summary.NewBySeverity), sevSummary(d.Summary.ResolvedBySeverity), len(d.Issues.Persisting))

	writeIssueSection(&b, "NEW issues (regressions)", d.Issues.New, max)
	writeIssueSection(&b, "RESOLVED issues (fixed)", d.Issues.Resolved, max)

	if len(d.Pages.Added) > 0 || len(d.Pages.Removed) > 0 {
		fmt.Fprintf(&b, "\nPages: +%d added, -%d removed\n", len(d.Pages.Added), len(d.Pages.Removed))
		writeURLSection(&b, "added", d.Pages.Added, max)
		writeURLSection(&b, "removed", d.Pages.Removed, max)
	}

	if delta := d.Summary.ByAnalyzer; len(delta) > 0 {
		fmt.Fprintf(&b, "\nBy analyzer (issue count change):\n")
		for _, kv := range sortedDeltas(delta) {
			fmt.Fprintf(&b, "  %-14s %+d\n", kv.key, kv.val)
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func writeIssueSection(b *strings.Builder, title string, issues []analyze.Issue, max int) {
	if len(issues) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s (%d):\n", title, len(issues))
	for i, is := range issues {
		if i >= max {
			fmt.Fprintf(b, "  … and %d more\n", len(issues)-max)
			break
		}
		fmt.Fprintf(b, "  [%s] %s/%s  %s\n", is.Severity, is.Analyzer, is.Code, is.URL)
	}
}

func writeURLSection(b *strings.Builder, label string, urls []string, max int) {
	if len(urls) == 0 {
		return
	}
	for i, u := range urls {
		if i >= max {
			fmt.Fprintf(b, "  … and %d more %s\n", len(urls)-max, label)
			break
		}
		fmt.Fprintf(b, "  %s %s\n", symbolFor(label), u)
	}
}

func symbolFor(label string) string {
	if label == "added" {
		return "+"
	}
	return "-"
}

// sevSummary renders a by-severity count map as "2 error, 1 warning" worst-first, or "0" when
// empty.
func sevSummary(m map[string]int) string {
	order := []analyze.Severity{analyze.Error, analyze.Warning, analyze.Info}
	var parts []string
	total := 0
	for _, s := range order {
		if n := m[string(s)]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s))
			total += n
		}
	}
	if total == 0 {
		return "0"
	}
	return strings.Join(parts, ", ")
}

type kv struct {
	key string
	val int
}

func sortedDeltas(m map[string]int) []kv {
	out := make([]kv, 0, len(m))
	for k, v := range m {
		out = append(out, kv{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
