// Package perf reports Core Web Vitals from headless-rendered pages. When rendering
// produced real measurements, it emits per-page findings against Google's published
// thresholds for LCP, FCP, CLS, TBT, and TTFB. In raw mode (no rendering) it falls
// back to a TTFB proxy from raw fetch duration and notes that full CWV requires
// headless mode.
package perf

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer evaluates Core Web Vitals collected during headless rendering.
type Analyzer struct{}

// New returns a performance analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "perf" }
func (Analyzer) Description() string {
	return "Core Web Vitals (LCP, FCP, CLS, TBT, TTFB) — populated when --render headless"
}

// Google CWV thresholds. Values at or below "good" pass; between good and "ni" need
// improvement; above "ni" are poor.
const (
	lcpGood, lcpNI   = 2500.0, 4000.0
	fcpGood, fcpNI   = 1800.0, 3000.0
	clsGood, clsNI   = 0.1, 0.25
	tbtGood, tbtNI   = 200.0, 600.0
	ttfbGood, ttfbNI = 800.0, 1800.0
)

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	rendered := false
	for _, p := range result.Pages {
		if p.Render != nil && p.Render.Implemented {
			rendered = true
			break
		}
	}
	if !rendered {
		return rawFallback(result)
	}

	var issues []analyze.Issue
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		r := p.Render
		if r == nil {
			continue
		}
		if !r.Implemented {
			if r.Note != "" {
				issues = append(issues, analyze.Issue{
					Analyzer: "perf", URL: p.FinalURL, Severity: analyze.Info,
					Code: "perf-cwv-render-failed", Message: "Headless rendering failed; CWV not collected",
					Data: map[string]any{"note": r.Note},
				})
			}
			continue
		}

		if r.RawFallback {
			issues = append(issues, analyze.Issue{
				Analyzer: "perf", URL: p.FinalURL, Severity: analyze.Warning,
				Code: "perf-render-incomplete", Message: "Headless render finished thinner than the raw HTML, so the page likely had not finished rendering; the raw HTML was analyzed instead and Core Web Vitals may be unreliable for this page",
				Data: map[string]any{"rendered_bytes": r.RenderedBytes, "raw_bytes": r.RawBytes},
			})
		}

		issues = append(issues, analyze.Issue{
			Analyzer: "perf", URL: p.FinalURL, Severity: analyze.Info,
			Code: "perf-cwv-measured", Message: "Core Web Vitals measured (lab mode)",
			Data: map[string]any{
				"lcp_ms": r.LCP, "fcp_ms": r.FCP, "cls": r.CLS,
				"tbt_ms": r.TBT, "ttfb_ms": r.TTFB,
			},
		})

		issues = append(issues, scoreMS(p.FinalURL, "lcp", r.LCP, lcpGood, lcpNI,
			"Largest Contentful Paint")...)
		issues = append(issues, scoreMS(p.FinalURL, "fcp", r.FCP, fcpGood, fcpNI,
			"First Contentful Paint")...)
		issues = append(issues, scoreCLS(p.FinalURL, r.CLS)...)
		issues = append(issues, scoreMS(p.FinalURL, "tbt", r.TBT, tbtGood, tbtNI,
			"Total Blocking Time (lab proxy for INP)")...)
		issues = append(issues, scoreMS(p.FinalURL, "ttfb", r.TTFB, ttfbGood, ttfbNI,
			"Time to First Byte")...)
	}
	return issues
}

func scoreMS(url, key string, val, good, ni float64, label string) []analyze.Issue {
	if val <= 0 {
		return nil
	}
	switch {
	case val <= good:
		return nil
	case val <= ni:
		return []analyze.Issue{{
			Analyzer: "perf", URL: url, Severity: analyze.Warning,
			Code:    "perf-" + key + "-needs-improvement",
			Message: label + " needs improvement",
			Data:    map[string]any{"value_ms": val, "threshold_ms": good},
		}}
	default:
		return []analyze.Issue{{
			Analyzer: "perf", URL: url, Severity: analyze.Error,
			Code:    "perf-" + key + "-poor",
			Message: label + " is poor",
			Data:    map[string]any{"value_ms": val, "threshold_ms": ni},
		}}
	}
}

func scoreCLS(url string, val float64) []analyze.Issue {
	if val <= 0 {
		return nil
	}
	switch {
	case val <= clsGood:
		return nil
	case val <= clsNI:
		return []analyze.Issue{{
			Analyzer: "perf", URL: url, Severity: analyze.Warning,
			Code:    "perf-cls-needs-improvement",
			Message: "Cumulative Layout Shift needs improvement",
			Data:    map[string]any{"value": val, "threshold": clsGood},
		}}
	default:
		return []analyze.Issue{{
			Analyzer: "perf", URL: url, Severity: analyze.Error,
			Code:    "perf-cls-poor",
			Message: "Cumulative Layout Shift is poor",
			Data:    map[string]any{"value": val, "threshold": clsNI},
		}}
	}
}

// rawFallback retains the original stub behavior when no page was rendered: a single
// info noting CWV requires headless mode, plus a per-page TTFB proxy from raw fetch
// duration so the analyzer is still useful.
func rawFallback(result *crawler.Result) []analyze.Issue {
	issues := []analyze.Issue{{
		Analyzer: "perf", URL: result.Seed, Severity: analyze.Info,
		Code:    "perf-cwv-not-collected",
		Message: "Core Web Vitals require headless rendering. Run with --render headless.",
	}}
	for _, p := range result.Pages {
		if p.StatusCode == 200 && p.Duration > 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "perf", URL: p.FinalURL, Severity: analyze.Info,
				Code: "perf-response-time", Message: "Server response time (TTFB proxy)",
				Data: map[string]any{"duration_ms": p.Duration.Milliseconds()},
			})
		}
	}
	return issues
}
