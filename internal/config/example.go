package config

import (
	"fmt"
	"os"
)

// ExampleYAML is a fully-commented starter configuration written by `gocrawl init`.
const ExampleYAML = `# gocrawl configuration file
#
# All values are optional and fall back to built-in defaults. Command-line flags and
# GOCRAWL_* environment variables override anything set here.
#
# Run a crawl with:  gocrawl crawl https://example.com --config gocrawl.yaml

# Seed URL to start from (the positional CLI argument overrides this).
seed: "https://example.com"

# Rendering mode: "raw" (HTTP fetch, fast) or "headless" (chromedp — renders JS and captures
# Core Web Vitals; needs a Chromium browser installed).
render: "raw"

crawl:
  max_depth: 2          # link hops from the seed (0 = unlimited; bounded by max_pages instead)
  max_pages: 500        # hard cap on the number of pages crawled
  concurrency: 4        # number of parallel fetch workers
  rate_per_second: 0    # max requests/second across the crawl (0 = unlimited)
  adaptive_delay: true  # slow down automatically on HTTP 429/503 responses
  user_agent: "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)"
  timeout: "15s"        # per-request timeout
  max_body_bytes: 5242880  # 5 MiB cap on a single response body
  respect_robots: true  # obey robots.txt while crawling
  allow_subdomains: false  # follow links to subdomains of the seed host
  follow_external: false   # crawl links that leave the seed host
  follow_nofollow: false   # follow links marked rel="nofollow"
  strip_query: false       # ignore query strings (treat ?a=1 and ?a=2 as one URL).
                           # NOTE: this drops query params, so the query-dependent analyzers
                           # (utm, landing, wordpress) are automatically skipped while it is on.
  include: []           # only crawl URLs matching at least one of these regexes
  exclude:              # never crawl URLs matching any of these regexes
    - "\\.(?:png|jpe?g|gif|svg|webp|ico|css|js|pdf|zip)(?:\\?|$)"

output:
  format: "json"        # "json", "csv", or "html"
  path: ""              # file to write to; empty = stdout

analyzers:
  # If "enabled" is non-empty, only those analyzers run. Otherwise all run except those
  # listed in "disabled". Names: seo, redirects, links, robots, sitemap, structured, perf,
  # images, urls, security, pagination, hreflang, amp, duplicates, content, botwall,
  # wordpress, the SEA analyzers utm, tracking, datalayer, landing, and the AI-search
  # analyzers aeo, geo.
  enabled: []
  disabled: []
  # Turn on the opt-in specialized checks (off by default): the AI-search heuristics and
  # the WordPress security-endpoint probes.
  # aeo-no-answer-lead and geo-low-quotable-density.
  specialized: false

store:
  # Where 'gocrawl crawl --save' writes crawls and where 'gocrawl history' / 'gocrawl
  # compare' read them from. Empty = ~/.gocrawl/crawls. Saved crawls are addressable by
  # their "<host>/<timestamp>" ID, or by "latest" / a bare host name.
  dir: ""
`

// WriteExample writes the example configuration to path, refusing to overwrite an existing
// file.
func WriteExample(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite", path)
	}
	return os.WriteFile(path, []byte(ExampleYAML), 0o644)
}
