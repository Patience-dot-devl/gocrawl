package links

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

const (
	// maxExternalChecks caps how many distinct URLs one crawl checks. A site that links to
	// more is checked for the first maxExternalChecks in sorted order, and the summary says
	// how many were left out.
	maxExternalChecks = 1000
	// externalWorkers is how many checks run at once. The governed fetcher still applies the
	// crawl's rate limit and each host's Crawl-delay, so this only overlaps waiting on slow hosts.
	externalWorkers = 8
	// maxSourceExamples caps the linking pages listed per broken target.
	maxSourceExamples = 10
)

// Option configures the links analyzer.
type Option func(*Analyzer)

// WithExternalCheck turns on the opt-in check of outbound links and page resources (images,
// scripts, stylesheets): each distinct URL is requested once through fetcher, with HEAD where
// fetcher supports it. runner.Run passes the governed analyzer fetcher, so the checks obey the
// crawl's rate limit and robots.txt, and credentials stay scoped to the seed host. A nil
// fetcher leaves the check off.
func WithExternalCheck(fetcher crawler.Fetcher) Option {
	return func(a *Analyzer) { a.check = fetcher }
}

// checkTarget is one URL to check and where the crawl found it.
type checkTarget struct {
	url     string
	kind    string // "link", or the resource kind: "image", "script", "stylesheet"
	sources map[string]bool
}

// outcome classifies a checked URL.
type outcome int

const (
	outcomeOK outcome = iota
	outcomeBroken
	outcomeBlocked // 401/403/429/999: the server refused a bot, which says nothing about the link
	outcomeRobots  // robots.txt disallows checking it
)

type checkResult struct {
	outcome outcome
	status  int
	err     string
}

// checkExternal finds the outbound links and the page resources the crawl did not fetch,
// checks each distinct URL once, and reports the broken ones: one issue per broken URL, on the
// first page (sorted) that uses it, with how many pages do and examples. A summary counts what
// was checked, skipped and left out.
func (a Analyzer) checkExternal(ctx context.Context, result *crawler.Result) []analyze.Issue {
	targets := collectTargets(result)
	if len(targets) == 0 {
		return nil
	}
	urls := make([]string, 0, len(targets))
	for u := range targets {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	overLimit := 0
	if len(urls) > maxExternalChecks {
		overLimit = len(urls) - maxExternalChecks
		urls = urls[:maxExternalChecks]
	}

	results := make(map[string]checkResult, len(urls))
	var mu sync.Mutex
	jobs := make(chan string)
	var wg sync.WaitGroup
	for range min(externalWorkers, len(urls)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				r := a.checkURL(ctx, u)
				mu.Lock()
				results[u] = r
				mu.Unlock()
			}
		}()
	}
	for _, u := range urls {
		if ctx.Err() != nil {
			break
		}
		jobs <- u
	}
	close(jobs)
	wg.Wait()

	var issues []analyze.Issue
	var broken, blocked, robots int
	for _, u := range urls {
		r, ok := results[u]
		if !ok {
			continue // the crawl was cancelled before this one ran
		}
		switch r.outcome {
		case outcomeBlocked:
			blocked++
			continue
		case outcomeRobots:
			robots++
			continue
		case outcomeOK:
			continue
		}
		broken++
		t := targets[u]
		sources := sortedKeys(t.sources)
		data := map[string]any{"target": u, "pages": len(sources), "examples": firstN(sources, maxSourceExamples)}
		if r.status != 0 {
			data["status"] = r.status
		} else {
			data["error"] = r.err
		}
		if t.kind == "link" {
			issues = append(issues, analyze.Issue{
				Analyzer: "links", URL: sources[0], Severity: analyze.Warning,
				Code: "link-external-broken", Message: "Outbound link is broken", Data: data,
			})
			continue
		}
		data["kind"] = t.kind
		issues = append(issues, analyze.Issue{
			Analyzer: "links", URL: sources[0], Severity: analyze.Error,
			Code: "resource-broken", Message: "Page loads a " + t.kind + " that is broken", Data: data,
		})
	}

	checked := len(results)
	issues = append(issues, analyze.Issue{
		Analyzer: "links", URL: analyze.SiteBase(result), Severity: analyze.Info,
		Code: "link-external-summary", Message: "Outbound link and resource check",
		Data: map[string]any{
			"checked":        checked - robots,
			"broken":         broken,
			"blocked":        blocked,
			"robots_skipped": robots,
			"over_limit":     overLimit,
		},
	})
	return issues
}

// checkURL requests u, with HEAD where the fetcher supports it. A HEAD the server rejects
// (403, 405, 501) is retried once as GET, since some servers answer HEAD differently.
func (a Analyzer) checkURL(ctx context.Context, u string) checkResult {
	head, canHead := a.check.(crawler.HeadFetcher)
	var page *crawler.Page
	var err error
	if canHead {
		page, err = head.Head(ctx, u)
		if page != nil && err == nil {
			switch page.StatusCode {
			case http.StatusForbidden, http.StatusMethodNotAllowed, http.StatusNotImplemented:
				page, err = a.check.Fetch(ctx, u)
			}
		}
	} else {
		page, err = a.check.Fetch(ctx, u)
	}
	if errors.Is(err, crawler.ErrRobotsDisallowed) {
		return checkResult{outcome: outcomeRobots}
	}
	if page == nil {
		msg := "request failed"
		if err != nil {
			msg = err.Error()
		}
		return checkResult{outcome: outcomeBroken, err: msg}
	}
	if page.StatusCode == 0 {
		msg := page.Err
		if msg == "" && err != nil {
			msg = err.Error()
		}
		return checkResult{outcome: outcomeBroken, err: msg}
	}
	return classify(page.StatusCode)
}

func classify(status int) checkResult {
	switch {
	case status < 400:
		return checkResult{outcome: outcomeOK, status: status}
	case status == http.StatusUnauthorized, status == http.StatusForbidden,
		status == http.StatusTooManyRequests, status == 999: // 999: LinkedIn's bot refusal
		return checkResult{outcome: outcomeBlocked, status: status}
	default:
		return checkResult{outcome: outcomeBroken, status: status}
	}
}

// collectTargets gathers the URLs to check: outbound <a href> links, and images, scripts and
// stylesheets the pages load, on any host. A URL the crawl itself fetched is skipped, since its
// status is already known (an internal one is reported by link-broken).
func collectTargets(result *crawler.Result) map[string]*checkTarget {
	targets := map[string]*checkTarget{}
	add := func(raw, kind, source string) {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return
		}
		u.Fragment = ""
		key := u.String()
		if _, crawled := result.Page(key); crawled {
			return
		}
		t := targets[key]
		if t == nil {
			t = &checkTarget{url: key, kind: kind, sources: map[string]bool{}}
			targets[key] = t
		} else if t.kind == "link" && kind != "link" {
			t.kind = kind // a URL used as a resource is reported as one
		}
		t.sources[source] = true
	}

	for _, p := range result.Pages {
		if !p.IsHTML() {
			continue
		}
		for _, l := range p.Links {
			if !l.External {
				continue
			}
			target := l.Resolved
			if target == "" {
				target = l.URL
			}
			add(target, "link", p.FinalURL)
		}
		p.Doc.Find("img[src], script[src], link[href]").Each(func(_ int, s *goquery.Selection) {
			kind, attr := resourceKind(s)
			if kind == "" {
				return
			}
			ref, _ := s.Attr(attr)
			ref = strings.TrimSpace(ref)
			if ref == "" || strings.HasPrefix(ref, "data:") {
				return
			}
			if _, resolved, _ := result.ResolveHref(p, ref); resolved != "" {
				add(resolved, kind, p.FinalURL)
			}
		})
	}
	return targets
}

// resourceKind names the resource an element loads, and the attribute holding its URL; "" for
// an element that loads nothing worth checking (a <link> that is not a stylesheet).
func resourceKind(s *goquery.Selection) (kind, attr string) {
	switch goquery.NodeName(s) {
	case "img":
		return "image", "src"
	case "script":
		return "script", "src"
	case "link":
		for _, rel := range strings.Fields(strings.ToLower(s.AttrOr("rel", ""))) {
			if rel == "stylesheet" {
				return "stylesheet", "href"
			}
		}
	}
	return "", ""
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
