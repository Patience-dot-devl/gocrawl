# Architecture & developer guide

gocrawl is built around one idea: **the crawl engine knows nothing about specific checks, and
the checks never fetch the crawl.** The two sides meet at a single interface,
`analyze.Analyzer`, which is what makes new SEO/SEA checks cheap to add.

## Data flow

```
seed URL + config
      │
      ▼
 config.Config ──► crawler.Options          (internal/config)
      │
      ▼
 crawler.Engine.Crawl()                      (internal/crawler)
   • concurrent fetch within scope
   • robots.txt + rate limiting
   • redirect capture, link extraction
      │
      ▼
 crawler.Result  (pages, redirects, robots)
      │
      ▼
 analyze.Run(analyzers, result)             (internal/analyze + internal/analyze/*)
   • each Analyzer emits []Issue
      │
      ▼
 report.Build(result, issues) ──► Report    (internal/report)
      │
      ▼
 JSON / CSV / HTML  (file, stdout, or MCP response)
```

[`runner.Run`](../internal/runner/runner.go) is the single orchestration point used by both
the CLI (`gocrawl crawl`) and the MCP server. It compiles the config into
`crawler.Options`, builds the engine and analyzer registry, runs the crawl, runs the selected
analyzers over the result, and hands everything to the report builder.

## Package map

| Package | Responsibility |
| --- | --- |
| [`cmd/gocrawl`](../cmd/gocrawl) | CLI (Cobra): `crawl`, `analyzers list`, `init`, `mcp`. |
| [`internal/config`](../internal/config) | Layered config (defaults → YAML → env → flags) and compilation into `crawler.Options`. |
| [`internal/crawler`](../internal/crawler) | Concurrent crawl engine, HTTP fetcher, robots.txt, URL normalization, scope rules, link extraction. Defines `Page`, `Link`, `Redirect`, `Result`, `Options`. |
| [`internal/render`](../internal/render) | Render-mode fetcher selection; headless rendering via chromedp (Core Web Vitals). |
| [`internal/analyze`](../internal/analyze) | The `Analyzer` interface, `Issue`/`Severity` types, and the `Registry`. |
| `internal/analyze/<name>` | One package per analyzer: `seo`, `httpx` (name `redirects`), `links`, `robotscheck` (name `robots`), `sitemap`, `structured`, `perf`, `images`, `urls`, `security`, `pagination`, `hreflang`, `amp`, `duplicates`, `content`, the CMS-specific `wordpress`, the SEA analyzers `utm`, `tracking`, `datalayer`, `landing`, and the AI-search analyzers `aeo`, `geo`. `seaurl` is a shared UTM-parsing helper (not an analyzer). |
| [`internal/runner`](../internal/runner) | Wires engine + registry + report into `Run`; also `BuildRegistry` and `ListAnalyzers`. |
| [`internal/report`](../internal/report) | Builds the `Report` and serializes it (JSON, CSV, HTML). |
| [`internal/mcpserver`](../internal/mcpserver) | Exposes `crawl` and `list_analyzers` over MCP. |

## The crawl engine

[`crawler.Engine.Crawl`](../internal/crawler/engine.go) runs a bounded, concurrent
breadth-first walk:

- A [`frontier`](../internal/crawler/frontier.go) — a FIFO queue shared by a fixed pool of
  `Concurrency` worker goroutines — holds discovered-but-not-yet-fetched URLs. Sizing the
  queue to the site rather than to concurrency (instead of spawning a goroutine per
  discovered URL) keeps the goroutine count bounded even with `MaxPages 0`, and pulling in
  FIFO order gives a true breadth-first traversal. A `golang.org/x/time/rate` limiter
  enforces `RatePerSecond` per fetch.
- `inScope` filters each candidate URL by scheme, host (seed host ± subdomains, unless
  `FollowExternal`), and the exclude-then-include regex rules.
- `RespectRobots` consults a per-host robots manager before fetching; `MaxPages` and
  `MaxDepth` bound the walk. `MaxDuration` bounds the whole crawl's wall-clock time via a
  context deadline.
- After fetching, links are extracted from the parsed document and pushed onto the frontier
  (skipping external links unless `FollowExternal`, and `nofollow` links unless
  `FollowNofollow`).
- `robots.txt` is collected for every crawled host afterward, so the `robots` analyzer has
  data even when `RespectRobots` is off.
- Stopping early (a canceled context, e.g. Ctrl-C or `MaxDuration`) still returns everything
  fetched so far as a result, flagged via `Result.Coverage.Interrupted` /
  `DurationLimitReached` rather than discarded as an error.

The engine produces a [`crawler.Result`](../internal/crawler/types.go): the list of pages
(each with status, redirects, links, timing, parsed doc), the per-host robots data, and an
index for looking up a page by URL.

## The analyzer seam

Every check implements [`analyze.Analyzer`](../internal/analyze/analyze.go):

```go
type Analyzer interface {
    Name() string
    Description() string
    Analyze(ctx context.Context, result *crawler.Result) []Issue
}
```

Analyzers are pure: they read the `Result` and return `Issue` values; they don't fetch,
mutate, or print. The [`Registry`](../internal/analyze/analyze.go) holds them in registration
order, and `Select(enabled, disabled)` decides which run for a given crawl (see
[Selecting analyzers](configuration.md#selecting-analyzers)). For per-page checks,
`analyze.EachPage(result, fn)` runs a function over every page and concatenates the issues.

## Adding an analyzer

1. Create a package under `internal/analyze/<yourcheck>/`.
2. Implement the `Analyzer` interface (use `analyze.EachPage` for per-page checks).
3. Register it in [`runner.BuildRegistry`](../internal/runner/runner.go).
4. Add a unit test with an HTML fixture under `testdata/`.

No engine changes are needed — this is exactly how the
[SEA analyzers](roadmap.md) (`utm`, `tracking`, `landing`) were added.
[CONTRIBUTING.md](../CONTRIBUTING.md#adding-a-new-analyzer) is the canonical guide for
contribution mechanics, coding guidelines, and the PR checklist.

## Build, test, lint

```sh
make build    # go build -o gocrawl ./cmd/gocrawl
make test     # go test ./...
make vet      # go vet ./...
make lint     # golangci-lint run
```

CI ([`.github/workflows/ci.yml`](../.github/workflows/ci.yml)) runs `go mod verify`,
`go vet`, `go build`, `go test -race`, and golangci-lint on every push and PR to `main`.
