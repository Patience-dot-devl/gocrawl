package security

import (
	"net/url"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// host is one crawled host's pages, in crawl order. Every audit check is a property of the
// host's server configuration rather than of an individual page, so each runs once over this
// group and reports against a single representative URL.
type host struct {
	name  string
	pages []*crawler.Page
}

// auditSite runs the opt-in audit checks once per crawled host. Hosts are visited in
// first-seen order so a report's findings are stable across runs.
func (a Analyzer) auditSite(result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	for _, h := range groupByHost(result) {
		issues = append(issues, h.auditTLS()...)
		issues = append(issues, h.auditCookies()...)
		issues = append(issues, h.auditHeaders()...)
	}
	return issues
}

// groupByHost buckets successfully fetched pages by their final host, preserving first-seen
// order. Pages that never produced a response are skipped: they carry no headers, no TLS
// state, and no cookies for the audit to read.
func groupByHost(result *crawler.Result) []host {
	var order []string
	byName := map[string][]*crawler.Page{}
	for _, p := range result.Pages {
		if p == nil || p.StatusCode == 0 || p.FinalURL == "" {
			continue
		}
		u, err := url.Parse(p.FinalURL)
		if err != nil || u.Host == "" {
			continue
		}
		if _, seen := byName[u.Host]; !seen {
			order = append(order, u.Host)
		}
		byName[u.Host] = append(byName[u.Host], p)
	}
	hosts := make([]host, 0, len(order))
	for _, name := range order {
		hosts = append(hosts, host{name: name, pages: byName[name]})
	}
	return hosts
}

// ref returns the URL audit findings for this host are reported against — the first crawled
// page on it. Every check is host-wide, so the exact page is arbitrary; what matters is that
// it is a real, clickable URL on the host in question.
func (h host) ref() string {
	if len(h.pages) == 0 {
		return "https://" + h.name + "/"
	}
	return h.pages[0].FinalURL
}

// schemes partitions the host's pages by the scheme they *ended* on, after redirects. A page
// counted as plaintext is one a visitor genuinely reads over http:// — an http URL that
// redirects to https lands in secure, which is the point of the redirect.
func (h host) schemes() (secure, plaintext []*crawler.Page) {
	for _, p := range h.pages {
		u, err := url.Parse(p.FinalURL)
		if err != nil {
			continue
		}
		switch u.Scheme {
		case "https":
			secure = append(secure, p)
		case "http":
			plaintext = append(plaintext, p)
		}
	}
	return secure, plaintext
}

// issue builds a host-level finding.
func (h host) issue(sev analyze.Severity, code, msg string, data map[string]any) analyze.Issue {
	return analyze.Issue{Analyzer: "security", URL: h.ref(), Severity: sev, Code: code, Message: msg, Data: data}
}
