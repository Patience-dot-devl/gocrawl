// Package sitemap discovers and parses sitemap.xml (including sitemap indexes) and
// cross-checks declared URLs against what was actually crawled.
package sitemap

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"io"
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

// Fetch retrieves and parses the sitemap(s) reachable from candidates (URL -> declared, where
// declared is true for sitemaps named in robots.txt or a parent index, false for a
// conventional path merely guessed at). It follows sitemap indexes up to two levels deep.
// parsed counts candidates that successfully parsed as an urlset or index (0 means nothing
// usable was found at any candidate); invalidDeclared lists declared candidates whose response
// parsed as neither (a guessed path that fails to parse is almost always a soft-404 and isn't
// reported); truncatedDeclared lists declared candidates whose body was cut short by the
// fetcher's size cap before it could be parsed, so they aren't misreported as invalid.
func Fetch(ctx context.Context, fetcher crawler.Fetcher, candidates map[string]bool) (urls map[string]bool, parsed int, invalidDeclared, truncatedDeclared []string) {
	urls = map[string]bool{}
	visited := map[string]bool{}

	var fetchOne func(smURL string, depth int, declared bool)
	fetchOne = func(smURL string, depth int, declared bool) {
		if depth > 2 || visited[smURL] {
			return
		}
		visited[smURL] = true

		page, ferr := fetcher.Fetch(ctx, smURL)
		if ferr != nil || page == nil || page.StatusCode != 200 || len(page.Body) == 0 {
			return
		}
		if page.Truncated {
			if declared {
				truncatedDeclared = append(truncatedDeclared, smURL)
			}
			return
		}
		body, gzErr := maybeGunzip(smURL, page)
		if gzErr != nil {
			if declared {
				invalidDeclared = append(invalidDeclared, smURL)
			}
			return
		}

		var idx sitemapindex
		if xml.Unmarshal(body, &idx) == nil && len(idx.Sitemaps) > 0 {
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
		if xml.Unmarshal(body, &us) == nil {
			parsed++
			for _, u := range us.URLs {
				if loc := strings.TrimSpace(u.Loc); loc != "" {
					urls[normalize(loc)] = true
				}
			}
			return
		}
		// The response did not parse as a sitemap. Only flag it when the sitemap was
		// explicitly declared (robots.txt or an index); a guessed conventional path that
		// returns HTML is a soft-404, not a broken sitemap, so reporting it is noise.
		if declared {
			invalidDeclared = append(invalidDeclared, smURL)
		}
	}

	for c, declared := range candidates {
		fetchOne(c, 0, declared)
	}
	return urls, parsed, invalidDeclared, truncatedDeclared
}

// maybeGunzip decompresses page.Body when smURL or the response's Content-Type indicates a
// gzip-compressed sitemap (e.g. sitemap.xml.gz), otherwise it returns the body unchanged.
func maybeGunzip(smURL string, page *crawler.Page) ([]byte, error) {
	isGzip := strings.HasSuffix(strings.ToLower(smURL), ".gz") ||
		strings.Contains(strings.ToLower(page.ContentType), "gzip")
	if !isGzip {
		return page.Body, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(page.Body))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
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

	sitemapURLs, parsed, invalidDeclared, truncatedDeclared := Fetch(ctx, a.fetcher, candidates)
	for _, u := range invalidDeclared {
		issues = append(issues, analyze.Issue{
			Analyzer: "sitemap", URL: u, Severity: analyze.Warning,
			Code: "sitemap-invalid", Message: "Could not parse sitemap as urlset or index",
		})
	}
	for _, u := range truncatedDeclared {
		issues = append(issues, analyze.Issue{
			Analyzer: "sitemap", URL: u, Severity: analyze.Warning,
			Code: "sitemap-truncated", Message: "Sitemap exceeds the crawler's fetch size limit and was cut off before it could be parsed",
		})
	}

	if parsed == 0 {
		if len(truncatedDeclared) == 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "sitemap", URL: base, Severity: analyze.Warning,
				Code: "sitemap-missing", Message: "No sitemap found at robots.txt or conventional locations",
			})
		}
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
