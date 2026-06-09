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

# Rendering mode: "raw" (HTTP fetch, fast) or "headless" (chromedp — currently stubbed,
# falls back to raw and annotates pages that rendering is not yet active).
render: "raw"

crawl:
  max_depth: 2          # link hops from the seed (0 = only the seed page)
  max_pages: 500        # hard cap on the number of pages crawled
  concurrency: 4        # number of parallel fetch workers
  rate_per_second: 0    # max requests/second across the crawl (0 = unlimited)
  user_agent: "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)"
  timeout: "15s"        # per-request timeout
  max_body_bytes: 5242880  # 5 MiB cap on a single response body
  respect_robots: true  # obey robots.txt while crawling
  allow_subdomains: false  # follow links to subdomains of the seed host
  follow_external: false   # crawl links that leave the seed host
  follow_nofollow: false   # follow links marked rel="nofollow"
  include: []           # only crawl URLs matching at least one of these regexes
  exclude:              # never crawl URLs matching any of these regexes
    - "\\.(?:png|jpe?g|gif|svg|webp|ico|css|js|pdf|zip)(?:\\?|$)"

output:
  format: "json"        # "json" or "csv"
  path: ""              # file to write to; empty = stdout

analyzers:
  # If "enabled" is non-empty, only those analyzers run. Otherwise all run except those
  # listed in "disabled". Names: seo, redirects, links, robots, sitemap, structured, perf,
  # and the SEA analyzers utm, tracking, landing.
  enabled: []
  disabled: []
`

// WriteExample writes the example configuration to path, refusing to overwrite an existing
// file.
func WriteExample(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite", path)
	}
	return os.WriteFile(path, []byte(ExampleYAML), 0o644)
}
