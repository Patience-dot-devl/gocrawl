// Package sitemap discovers and parses sitemap.xml (including sitemap indexes) and
// cross-checks declared URLs against what was actually crawled.
package sitemap

import (
	"context"
	"encoding/xml"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer fetches and validates sitemaps. It uses a Fetcher to retrieve sitemap files.
type Analyzer struct {
	fetcher crawler.Fetcher
}

// New returns a sitemap analyzer that fetches sitemaps with the given fetcher.
func New(fetcher crawler.Fetcher) *Analyzer { return &Analyzer{fetcher: fetcher} }

func (Analyzer) Name() string { return "sitemap" }
func (Analyzer) Description() string {
	return "sitemap.xml discovery/parsing (incl. index) and crawl-coverage cross-check"
}

type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

type sitemapindex struct {
	XMLName  xml.Name `xml:"sitemapindex"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	seed, err := url.Parse(result.Seed)
	if err != nil {
		return nil
	}
	var issues []analyze.Issue

	// Candidate sitemap URLs: those declared in robots.txt (declared==true) plus common
	// conventional paths we merely guess at (declared==false). The distinction matters for
	// error reporting: a declared sitemap that won't parse is a real misconfiguration worth
	// flagging, but a guessed path that returns non-XML is almost always a soft-404 (the
	// server answers 200 with an HTML page for any unknown path), so we stay silent about it.
	candidates := map[string]bool{}
	for _, data := range result.Robots {
		for _, sm := range data.Sitemaps {
			candidates[sm] = true
		}
	}
	base := seed.Scheme + "://" + seed.Host
	for _, path := range []string{"/sitemap.xml", "/sitemap_index.xml"} {
		if _, ok := candidates[base+path]; !ok {
			candidates[base+path] = false
		}
	}

	sitemapURLs := map[string]bool{}
	var parsed int
	visited := map[string]bool{}

	var fetchOne func(smURL string, depth int, declared bool)
	fetchOne = func(smURL string, depth int, declared bool) {
		if depth > 2 || visited[smURL] {
			return
		}
		visited[smURL] = true

		page, ferr := a.fetcher.Fetch(ctx, smURL)
		if ferr != nil || page == nil || page.StatusCode != 200 || len(page.Body) == 0 {
			return
		}

		var idx sitemapindex
		if xml.Unmarshal(page.Body, &idx) == nil && len(idx.Sitemaps) > 0 {
			parsed++
			for _, s := range idx.Sitemaps {
				if loc := strings.TrimSpace(s.Loc); loc != "" {
					// Children referenced by an index are declared by it.
					fetchOne(loc, depth+1, true)
				}
			}
			return
		}
		var us urlset
		if xml.Unmarshal(page.Body, &us) == nil {
			parsed++
			for _, u := range us.URLs {
				if loc := strings.TrimSpace(u.Loc); loc != "" {
					sitemapURLs[normalize(loc)] = true
				}
			}
			return
		}
		// The response did not parse as a sitemap. Only flag it when the sitemap was
		// explicitly declared (robots.txt or an index); a guessed conventional path that
		// returns HTML is a soft-404, not a broken sitemap, so reporting it is noise.
		if declared {
			issues = append(issues, analyze.Issue{
				Analyzer: "sitemap", URL: smURL, Severity: analyze.Warning,
				Code: "invalid-sitemap", Message: "Could not parse sitemap as urlset or index",
			})
		}
	}

	for c, declared := range candidates {
		fetchOne(c, 0, declared)
	}

	if parsed == 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "sitemap", URL: base, Severity: analyze.Warning,
			Code: "no-sitemap", Message: "No sitemap found at robots.txt or conventional locations",
		})
		return issues
	}

	// Cross-check coverage.
	crawled := map[string]bool{}
	for _, p := range result.Pages {
		if p.StatusCode == 200 {
			crawled[normalize(p.FinalURL)] = true
		}
	}

	var notInSitemap, notCrawled int
	for u := range crawled {
		if !sitemapURLs[u] {
			notInSitemap++
		}
	}
	for u := range sitemapURLs {
		if !crawled[u] {
			notCrawled++
		}
	}

	issues = append(issues, analyze.Issue{
		Analyzer: "sitemap", URL: base, Severity: analyze.Info,
		Code: "sitemap-coverage", Message: "Sitemap vs. crawl coverage",
		Data: map[string]any{
			"sitemap_urls":           len(sitemapURLs),
			"crawled_pages":          len(crawled),
			"crawled_not_in_sitemap": notInSitemap,
			"in_sitemap_not_crawled": notCrawled,
		},
	})
	return issues
}

func normalize(u string) string {
	u = strings.TrimSpace(u)
	if i := strings.Index(u, "#"); i >= 0 {
		u = u[:i]
	}
	if len(u) > 1 {
		u = strings.TrimRight(u, "/")
	}
	return u
}
