// Package sitemapgen turns a finished crawl into shareable site-map artifacts: a standard
// sitemap.xml (the sitemaps.org urlset) and a tree the HTML report draws as a node-link
// (org-chart-style) diagram, annotated with the analyzer issues found on each page.
//
// It is a pure transform over a crawler.Result and the analyzer issues — it never fetches or
// mutates anything. This keeps it outside the analyzer seam (analyzers emit Issues and must
// not write files); the runner invokes it as an optional side output of a crawl.
package sitemapgen

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Entry is one indexable URL destined for sitemap.xml.
type Entry struct {
	Loc     string `json:"loc"`               // canonical (post-redirect) URL
	LastMod string `json:"lastmod,omitempty"` // YYYY-MM-DD from the Last-Modified header, or "" if unknown
}

// PageIssue is an analyzer finding attached to a node, flattened to plain strings for the
// HTML template (decoupled from analyze.Issue).
type PageIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Analyzer string `json:"analyzer"`
	Message  string `json:"message"`
}

// SevCounts tallies findings by severity.
type SevCounts struct {
	Error   int `json:"error,omitempty"`
	Warning int `json:"warning,omitempty"`
	Info    int `json:"info,omitempty"`
}

func (c SevCounts) Total() int  { return c.Error + c.Warning + c.Info }
func (c SevCounts) Empty() bool { return c.Total() == 0 }

func (c *SevCounts) add(sev string) {
	switch sev {
	case string(analyze.Error):
		c.Error++
	case string(analyze.Warning):
		c.Warning++
	default:
		c.Info++
	}
}

func (c *SevCounts) addAll(o SevCounts) {
	c.Error += o.Error
	c.Warning += o.Warning
	c.Info += o.Info
}

// Node is a node in the site tree. Intermediate path segments that were never crawled
// directly (URL == "") still appear so the hierarchy stays connected.
type Node struct {
	Label    string      `json:"label"`              // path segment, e.g. "blog" or "/" for the root
	URL      string      `json:"url,omitempty"`      // crawled page URL, or "" for a synthetic intermediate node
	Title    string      `json:"title,omitempty"`    // <title> of the page, when available
	Status   int         `json:"status,omitempty"`   // HTTP status, 0 for synthetic nodes
	Depth    int         `json:"depth,omitempty"`    // crawl depth from the seed
	Issues   []PageIssue `json:"issues,omitempty"`   // findings on this exact page, worst-first
	Counts   SevCounts   `json:"counts,omitempty"`   // issue counts for this page only
	Subtotal SevCounts   `json:"subtotal,omitempty"` // issue counts for this node and all descendants
	Children []*Node     `json:"children,omitempty"` // sorted by Label
}

// Map is the generated site map: a flat list of entries for sitemap.xml and a tree for the
// HTML visualization, plus any findings that don't belong to a single crawled page.
type Map struct {
	Seed      string    `json:"seed"`
	Host      string    `json:"host"`
	Generated time.Time `json:"generated"`
	Entries   []Entry   `json:"entries,omitempty"`
	Root      *Node     `json:"root,omitempty"`
	// SiteWide holds issues whose URL didn't resolve to a crawled page in the tree (e.g. the
	// robots analyzer's per-host findings, keyed "host <hostname>").
	SiteWide []PageIssue `json:"site_wide,omitempty"`
	// Totals counts every issue placed on the tree plus the site-wide ones.
	Totals SevCounts `json:"totals,omitempty"`
}

// Generate builds a site map from a crawl result and its analyzer issues. Only successful
// (HTTP 200) HTML pages on the seed host (or its subdomains) are included in the tree and
// sitemap.xml; redirects, errors, assets, and off-site pages are excluded, matching what
// belongs in a sitemap. Issues are attached to the page they name (matched on URL); issues
// that name no crawled page land in SiteWide. The generated timestamp is passed in so callers
// stay deterministic and testable.
func Generate(result *crawler.Result, issues []analyze.Issue, generated time.Time) Map {
	m := Map{Seed: result.Seed, Generated: generated}

	seedHost := ""
	if u, err := url.Parse(result.Seed); err == nil {
		seedHost = strings.ToLower(u.Hostname())
	}
	m.Host = seedHost

	root := &Node{Label: "/"}
	byURL := map[string]*Node{} // normalized page URL -> node, for attaching issues

	seen := map[string]bool{}
	for _, p := range result.Pages {
		if p == nil || p.StatusCode != 200 || !p.IsHTML() {
			continue
		}
		loc := p.FinalURL
		if loc == "" {
			loc = p.RequestedURL
		}
		u, err := url.Parse(loc)
		if err != nil {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if seedHost != "" && host != seedHost && !strings.HasSuffix(host, "."+seedHost) {
			continue // off-site
		}
		if seen[loc] {
			continue
		}
		seen[loc] = true

		title := ""
		if p.Doc != nil {
			title = strings.TrimSpace(p.Doc.Find("title").First().Text())
		}
		m.Entries = append(m.Entries, Entry{Loc: loc, LastMod: lastMod(p)})
		node := insert(root, u, loc, title, p.StatusCode, p.Depth)
		if node != nil {
			byURL[normalizeKey(loc)] = node
		}
	}

	attachIssues(&m, byURL, issues)
	subtotals(root)

	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Loc < m.Entries[j].Loc })
	sortTree(root)
	m.Root = root
	return m
}

// attachIssues routes each issue to the node whose page it names, or to SiteWide when it
// names no crawled page, and tallies per-node and grand totals.
func attachIssues(m *Map, byURL map[string]*Node, issues []analyze.Issue) {
	for _, is := range issues {
		pi := PageIssue{Severity: string(is.Severity), Code: is.Code, Analyzer: is.Analyzer, Message: is.Message}
		m.Totals.add(pi.Severity)
		if node, ok := byURL[normalizeKey(is.URL)]; ok {
			node.Issues = append(node.Issues, pi)
			node.Counts.add(pi.Severity)
			continue
		}
		m.SiteWide = append(m.SiteWide, pi)
	}
	for _, node := range byURL {
		sortIssues(node.Issues)
	}
	sortIssues(m.SiteWide)
}

// sortIssues orders findings worst-first (error, warning, info) then by code, so the most
// important problem on a page is what the reader sees first.
func sortIssues(is []PageIssue) {
	sort.SliceStable(is, func(i, j int) bool {
		ri, rj := sevRank(is[i].Severity), sevRank(is[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return is[i].Code < is[j].Code
	})
}

func sevRank(s string) int {
	switch s {
	case string(analyze.Error):
		return 0
	case string(analyze.Warning):
		return 1
	default:
		return 2
	}
}

// subtotals computes each node's Subtotal (its own counts plus every descendant's) so a
// collapsed branch still reveals how many issues hide beneath it.
func subtotals(n *Node) SevCounts {
	total := n.Counts
	for _, c := range n.Children {
		total.addAll(subtotals(c))
	}
	n.Subtotal = total
	return total
}

// insert places a page into the tree under its path segments, creating intermediate nodes as
// needed, and returns the leaf node for that page. The root represents the host; query
// strings and fragments are ignored for hierarchy purposes (the full URL is still kept).
func insert(root *Node, u *url.URL, loc, title string, status, depth int) *Node {
	segments := splitPath(u.Path)
	node := root
	if len(segments) == 0 {
		// The seed/home page maps onto the root node itself.
		if node.URL == "" {
			node.URL, node.Title, node.Status, node.Depth = loc, title, status, depth
		}
		return node
	}
	for i, seg := range segments {
		child := findChild(node, seg)
		if child == nil {
			child = &Node{Label: seg}
			node.Children = append(node.Children, child)
		}
		if i == len(segments)-1 {
			child.URL, child.Title, child.Status, child.Depth = loc, title, status, depth
		}
		node = child
	}
	return node
}

func findChild(n *Node, label string) *Node {
	for _, c := range n.Children {
		if c.Label == label {
			return c
		}
	}
	return nil
}

func splitPath(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

func sortTree(n *Node) {
	sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Label < n.Children[j].Label })
	for _, c := range n.Children {
		sortTree(c)
	}
}

// normalizeKey canonicalizes a URL for matching issues to pages: trim space, drop the
// fragment, and strip a single trailing slash so "https://x/a" and "https://x/a/" match.
func normalizeKey(u string) string {
	u = strings.TrimSpace(u)
	if i := strings.IndexByte(u, '#'); i >= 0 {
		u = u[:i]
	}
	if len(u) > 1 {
		u = strings.TrimRight(u, "/")
	}
	return u
}

// lastMod returns the page's Last-Modified header formatted as YYYY-MM-DD, or "" if the
// header is missing or unparseable. Crawl time is deliberately not used as a fallback: it
// reflects when we fetched the page, not when its content changed, which would mislead
// search engines that read <lastmod>.
func lastMod(p *crawler.Page) string {
	if p.Header == nil {
		return ""
	}
	v := p.Header.Get("Last-Modified")
	if v == "" {
		return ""
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}
