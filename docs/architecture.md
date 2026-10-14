---
id: "architecture"
title: "Architecture"
scope: "How the engine, analyzer pipeline, and report builder fit together; the package map; adding an analyzer."
topic: "reference"
status: "current"
superseded_by: []
updated: 2026-10-13
depends_on: ["web.md", "configuration.md", "roadmap.md"]
sources: ["../internal/runner/runner.go", "../internal/runner/options.go", "../cmd/gocrawl", "../internal/config", "../internal/crawler", "../internal/render", "../internal/analyze", "../internal/runner", "../internal/report", "../internal/mcpserver", "../internal/crawlrequest", "../internal/webserver", "../internal/crawler/engine.go", "../internal/crawler/frontier.go", "../internal/crawler/progress.go", "../internal/crawler/types.go", "../internal/analyze/analyze.go", "../.github/workflows/ci.yml"]
---
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
 JSON / CSV / HTML  (file, stdout, MCP response, or web API response)
```

[`runner.Run`](../internal/runner/runner.go) is the single orchestration point used by the
CLI (`gocrawl crawl`), the MCP server, and the web API. It compiles the config into
`crawler.Options`, builds the engine and analyzer registry, runs the crawl, runs the selected
analyzers over the result, and hands everything to the report builder. A caller that needs
more than the finished report passes `RunOption` values
([`internal/runner/options.go`](../internal/runner/options.go)) instead of new parameters;
the CLI and MCP server pass none.

## Package map

| Package | Responsibility |
| --- | --- |
| [`cmd/gocrawl`](../cmd/gocrawl) | CLI (Cobra): `crawl`, `analyzers list`, `init`, `mcp`, `serve`. |
| [`internal/config`](../internal/config) | Layered config (defaults → YAML → env → flags) and compilation into `crawler.Options`. |
| [`internal/crawler`](../internal/crawler) | Concurrent crawl engine, HTTP fetcher, robots.txt, URL normalization, scope rules, link extraction. Defines `Page`, `Link`, `Redirect`, `Result`, `Options`. |
| [`internal/render`](../internal/render) | Render-mode fetcher selection; headless rendering via chromedp (Core Web Vitals). |
| [`internal/analyze`](../internal/analyze) | The `Analyzer` interface, `Issue`/`Severity` types, and the `Registry`. |
| `internal/analyze/<name>` | One package per analyzer: `seo`, `httpx` (name `redirects`), `links`, `robotscheck` (name `robots`), `sitemap`, `structured`, `perf`, `images`, `urls`, `security`, `pagination`, `hreflang`, `amp`, `duplicates`, `content`, `botwall`, the CMS-specific `wordpress` and `shopify`, the SEA analyzers `utm`, `tracking`, `datalayer`, `landing`, `consent`, and the AI-search analyzers `aeo`, `geo`. `seaurl` is a shared UTM-parsing helper and `schemaorg` a shared JSON-LD graph parser (neither is an analyzer). |
| [`internal/runner`](../internal/runner) | Wires engine + registry + report into `Run`; also `BuildRegistry` and `ListAnalyzers`. |
| [`internal/report`](../internal/report) | Builds the `Report` and serializes it (JSON, CSV, HTML). |
| [`internal/mcpserver`](../internal/mcpserver) | Exposes `crawl` and `list_analyzers` over MCP. |
| [`internal/crawlrequest`](../internal/crawlrequest) | Maps a user-facing crawl request onto `config.Config`; the mapping shared by the MCP `crawl` tool and the web API. |
| [`internal/webserver`](../internal/webserver) | REST API (start/poll/cancel/export crawls) plus the embedded web UI; see [Web UI guide](web.md). |

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
  `MaxDepth` bound the walk. `MaxDuration` bounds the
  crawl's active time: a timer that stops while the crawl is paused cancels the crawl when the
  budget is used up.
- After fetching, links are extracted from the parsed document and pushed onto the frontier
  (skipping external links unless `FollowExternal`, and `nofollow` links unless
  `FollowNofollow`).
- `robots.txt` is collected for every crawled host afterward, so the `robots` analyzer has
  data even when `RespectRobots` is off.
- Analyzers that fetch extra resources (`geo`'s `llms.txt`, the `--specialized` `wordpress` and
  `shopify` probes) get their fetcher from `Engine.Govern`,
  which puts every request under the same rate limiter and `RespectRobots` policy as the
  crawl. A robots-disallowed probe returns `crawler.ErrRobotsDisallowed` without touching the
  server, and a `429` seen by a probe backs off the shared adaptive limiter. The `sitemap`
  analyzer fetches nothing after the crawl: it works from the pre-pass set (see below).
- `Options.OnProgress`, when set, is called after each page is recorded with a
  [`crawler.Progress`](../internal/crawler/progress.go) snapshot: pages recorded, queue length,
  the page just recorded, the rate limit in effect, and 429/503 responses in the last minute.
  Calls are serialized in recording order and run while the engine holds its locks, so the
  callback must return quickly and never call back into the engine.
- `Engine.Pause` and `Engine.Resume(rate)` hold back and release the frontier: in-flight
  requests finish and are recorded, a task that was waiting on the rate limiter goes back to
  the head of the queue, and cancellation still wakes a paused crawl. With
  `Options.PauseOnThrottle` the engine pauses itself after sustained 429/503 responses.
  `runner.Control` (through `runner.WithControl`) forwards pause and resume to the engine,
  queueing a pause made during the sitemap pre-pass.
- Stopping early (a canceled context, e.g. Ctrl-C or `MaxDuration`) still returns everything
  fetched so far as a result, flagged via `Result.Coverage.Interrupted` /
  `DurationLimitReached` rather than discarded as an error.

The engine produces a [`crawler.Result`](../internal/crawler/types.go): the list of pages
(each with status, redirects, links, timing, parsed doc), the per-host robots data, and an
index for looking up a page by URL.

Before the crawl, [`runner.Run`](../internal/runner/runner.go) runs a sitemap pre-pass:
`sitemap.DiscoverAll` on the governed analyzer fetcher, filtered through `Engine.InScope`. The
`sitemap` analyzer gets the result through `sitemap.WithURLs`, so the sitemap is fetched once
per crawl, and `Coverage.SitemapTotal` / `SitemapFetched` come from it. `runner.WithProgress`
wraps the engine callback into a `runner.Progress` that adds the run phase (`sitemap`,
`crawling`, `analyzing`) and the sitemap counts; the web server uses it for live progress.

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
