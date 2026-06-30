// Package report builds and serializes crawl reports (JSON, CSV, and HTML).
package report

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

//go:embed report.html.tmpl
var htmlTemplateFS embed.FS

// Report is the serializable result of a crawl plus analysis.
type Report struct {
	Seed         string          `json:"seed"`
	StartedAt    string          `json:"started_at"`
	FinishedAt   string          `json:"finished_at"`
	PagesCrawled int             `json:"pages_crawled"`
	Summary      Summary         `json:"summary"`
	Issues       []analyze.Issue `json:"issues"`
	// Notes carries human-readable advisories about the run itself (e.g. analyzers skipped
	// because of a conflicting option), not page findings. Omitted when empty.
	Notes []string `json:"notes,omitempty"`
	// SiteMap is the crawled site as a tree, annotated with the issues found on each page. It
	// powers the "Site map" tab of the HTML report and the optional sitemap.xml side output.
	// It is serialized so a JSON report is a complete artifact that `gocrawl render` can turn
	// back into HTML (including the Site map tab) without recrawling.
	SiteMap *sitemapgen.Map `json:"site_map,omitempty"`
}

// Summary aggregates issue counts.
type Summary struct {
	BySeverity map[string]int `json:"by_severity"`
	ByAnalyzer map[string]int `json:"by_analyzer"`
	ByStatus   map[string]int `json:"pages_by_status"`
}

// Build assembles a Report from a crawl Result and its issues.
func Build(result *crawler.Result, issues []analyze.Issue) *Report {
	if issues == nil {
		issues = []analyze.Issue{}
	}
	sum := Summary{
		BySeverity: map[string]int{},
		ByAnalyzer: map[string]int{},
		ByStatus:   map[string]int{},
	}
	for _, is := range issues {
		sum.BySeverity[string(is.Severity)]++
		sum.ByAnalyzer[is.Analyzer]++
	}
	for _, p := range result.Pages {
		sum.ByStatus[fmt.Sprintf("%d", p.StatusCode)]++
	}
	sm := sitemapgen.Generate(result, issues, result.Finished)
	return &Report{
		Seed:         result.Seed,
		StartedAt:    result.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		FinishedAt:   result.Finished.Format("2006-01-02T15:04:05Z07:00"),
		PagesCrawled: len(result.Pages),
		Summary:      sum,
		Issues:       issues,
		SiteMap:      &sm,
	}
}

// Reporter serializes a Report to a writer.
type Reporter interface {
	Write(w io.Writer, r *Report) error
}

// For returns the Reporter for the given format ("json", "csv", or "html"; default json).
func For(format string) Reporter {
	switch strings.ToLower(format) {
	case "csv":
		return CSVReporter{}
	case "html":
		return HTMLReporter{}
	default:
		return JSONReporter{}
	}
}

// JSONReporter writes an indented JSON report.
type JSONReporter struct{}

// Write implements Reporter.
func (JSONReporter) Write(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// CSVReporter writes one row per issue.
type CSVReporter struct{}

// Write implements Reporter.
func (CSVReporter) Write(w io.Writer, r *Report) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"analyzer", "severity", "code", "url", "message", "data"}); err != nil {
		return err
	}
	for _, is := range r.Issues {
		data := ""
		if len(is.Data) > 0 {
			b, _ := json.Marshal(is.Data)
			data = string(b)
		}
		if err := cw.Write([]string{is.Analyzer, string(is.Severity), is.Code, is.URL, is.Message, data}); err != nil {
			return err
		}
	}
	return cw.Error()
}

// HTMLReporter writes a self-contained HTML report (inline CSS, no external assets) suitable
// for opening in a browser or sharing as an artifact.
type HTMLReporter struct{}

// Write implements Reporter.
func (HTMLReporter) Write(w io.Writer, r *Report) error {
	tmpl, err := template.New("report.html.tmpl").Funcs(template.FuncMap{
		"severityClass": severityClass,
		"statusClass":   statusClass,
		"dataJSON":      dataJSON,
		"explain":       explain,
	}).ParseFS(htmlTemplateFS, "report.html.tmpl")
	if err != nil {
		return fmt.Errorf("parse html template: %w", err)
	}
	return tmpl.Execute(w, r)
}

func severityClass(s string) string {
	switch strings.ToLower(s) {
	case "error":
		return "sev-error"
	case "warning":
		return "sev-warning"
	default:
		return "sev-info"
	}
}

// statusClass maps an HTTP status to a CSS class for the site-map tree (0 = synthetic node).
func statusClass(status int) string {
	switch {
	case status == 0:
		return "st-none"
	case status >= 200 && status < 300:
		return "st-ok"
	case status >= 300 && status < 400:
		return "st-redirect"
	default:
		return "st-error"
	}
}

func dataJSON(d map[string]any) (string, error) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SummaryLines returns a short human-readable summary for stderr.
func (r *Report) SummaryLines() []string {
	lines := []string{
		fmt.Sprintf("Crawled %d pages from %s", r.PagesCrawled, r.Seed),
		fmt.Sprintf("Issues: %d (errors %d, warnings %d, info %d)",
			len(r.Issues), r.Summary.BySeverity["error"], r.Summary.BySeverity["warning"], r.Summary.BySeverity["info"]),
	}
	if len(r.Summary.ByAnalyzer) > 0 {
		keys := make([]string, 0, len(r.Summary.ByAnalyzer))
		for k := range r.Summary.ByAnalyzer {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", k, r.Summary.ByAnalyzer[k]))
		}
		lines = append(lines, "By analyzer: "+strings.Join(parts, " "))
	}
	return lines
}
