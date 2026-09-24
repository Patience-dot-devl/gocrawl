# CLAUDE.md

Guidance for agents working in this repository.

## What gocrawl is

`gocrawl` is a free, open-source website crawler for **SEO** and **SEA** audits, written in
Go. It walks a website concurrently and runs a pipeline of pluggable **analyzers** over every
page — checking technical SEO, redirects, broken links, `robots.txt`, `sitemap.xml`
coverage, structured data, Core Web Vitals, AI-search readiness, and more — then writes a
JSON, CSV, or HTML report.

It ships as a single static binary (no runtime deps) and can also run as an MCP
(Model Context Protocol) server over stdio so agentic tools can drive crawls, or as a web
application (`gocrawl serve`) with a REST API and an embedded browser UI.

Module path: `github.com/Patience-dot-devl/gocrawl`. Requires Go 1.26+.

## The core design idea

**The crawl engine knows nothing about specific checks, and the checks never fetch the
crawl.** The two sides meet at a single interface, `analyze.Analyzer`. This seam is the whole
point of the project: new SEO/SEA/AI-search checks slot in as independent analyzers without
ever touching the engine.

Analyzers are **pure**: they read a `crawler.Result` and return `[]analyze.Issue`. They must
not fetch, mutate shared state, or print. (Exception: a few analyzers like `sitemap`, `geo`,
`wordpress`, and `shopify` are constructed with a `crawler.Fetcher` so they can pull a small
number of extra resources such as `sitemap.xml`, `llms.txt`, or `/products.json` — these still
emit Issues, never print. `runner.Run` hands them a fetcher wrapped by `Engine.Govern`, so those
extra fetches obey the crawl's rate limiter and robots.txt policy.)

## Data flow

```
seed URL + config
  └─► config.Config ──► crawler.Options          (internal/config)
        └─► crawler.Engine.Crawl()               (internal/crawler)
              • concurrent fetch within scope, robots.txt + rate limiting,
                redirect capture, link extraction
              └─► crawler.Result (pages, redirects, robots, URL index)
                    └─► analyze.Run(analyzers, result)   (internal/analyze + .../<name>)
                          • each Analyzer emits []Issue
                          └─► report.Build(result, issues) ──► Report  (internal/report)
                                └─► JSON / CSV / HTML (file, stdout, MCP response, or web API response)
```

`runner.Run` (`internal/runner/runner.go`) is the **single orchestration point** used by the
CLI (`gocrawl crawl`), the MCP server, and the web API. It compiles config into
`crawler.Options`, builds the engine and analyzer registry, runs the crawl, runs the selected
analyzers, and hands everything to the report builder.

## Package map

| Package | Responsibility |
| --- | --- |
| `cmd/gocrawl` | CLI (Cobra): `crawl`, `render` (re-emit a saved JSON report as HTML/CSV without recrawling), `analyzers list`, `init`, `mcp`, `serve`, interactive mode. |
| `internal/config` | Layered config (defaults → YAML → env → flags) compiled into `crawler.Options`. |
| `internal/crawler` | Concurrent crawl engine, HTTP fetcher, robots.txt, URL normalization, scope rules, link extraction. Defines `Page`, `Link`, `Redirect`, `Result`, `Options`, `Fetcher`. |
| `internal/render` | Render-mode fetcher selection; headless rendering via chromedp (Core Web Vitals). |
| `internal/analyze` | The `Analyzer` interface, `Issue`/`Severity` types, `Registry`, and the `EachPage` helper. |
| `internal/analyze/schemaorg` | Shared JSON-LD parser: flattens a page's `ld+json` into an addressable node graph (`Node`, `Graph`, dotted paths, `@id` resolution). Not an analyzer — the same role `seaurl` plays for UTM parsing. |
| `internal/analyze/shopify` | Shopify detection, URL-template classification, per-template structured-data coverage, theme/app schema conflicts, variant modelling, and Shopify-specific crawl-hygiene checks. |
| `internal/analyze/<name>` | One package per analyzer (see below). |
| `internal/runner` | Wires engine + registry + report into `Run`; also `BuildRegistry` and `ListAnalyzers`. |
| `internal/report` | Builds the `Report` and serializes it (JSON, CSV, HTML); issue explanations live here. |
| `internal/mcpserver` | Exposes `crawl` and `list_analyzers` over MCP. |
| `internal/crawlrequest` | Maps a user-facing crawl request onto `config.Config`; the mapping shared by the MCP `crawl` tool and the web API. |
| `internal/webserver` | REST API (start/poll/cancel/export crawls) plus the embedded web UI (built from `web/`, see `docs/web.md`). |

## The analyzer seam

Every check implements:

```go
type Analyzer interface {
    Name() string
    Description() string
    Analyze(ctx context.Context, result *crawler.Result) []Issue
}
```

The `Registry` holds analyzers in **registration order** (set in `runner.BuildRegistry`,
`internal/runner/runner.go`) and `Select(enabled, disabled)` decides which run for a given
crawl. For per-page checks, use `analyze.EachPage(result, fn)`.

`Issue` carries `Analyzer`, `URL`, `Severity` (`info` / `warning` / `error`), a stable
`Code` string, a human `Message`, and optional `Data`. Keep `Code` values stable — they are
part of the report contract and have explanations in `internal/report/explanations.go`.

Registered analyzers (in order): `seo`, `redirects` (pkg `httpx`), `links`, `robots` (pkg
`robotscheck`), `sitemap`, `structured`, `perf`, `images`, `urls`, `security`, `pagination`,
`hreflang`, `amp`, `duplicates`, `content`, `botwall` (CAPTCHA / bot-challenge detection),
the CMS-specific `wordpress` and `shopify`, the SEA analyzers `utm` / `tracking` / `datalayer` / `landing` /
`consent`, and the AI-search analyzers `aeo` (Answer Engine Optimization) / `geo` (Generative
Engine Optimization). `seaurl` is a shared UTM-parsing helper and `schemaorg` a shared JSON-LD
parser, **not** analyzers.

`consent` leans on a property of the crawl rather than on markup alone: gocrawl never clicks a
consent banner, so every page it fetches is a pre-consent visit, and any tracking cookie or
beacon it observes was served without consent. Under `--render headless` it reads the
browser's real cookie jar (`Page.Render.Cookies`, captured via CDP) and the outbound requests
the render saw; in raw mode it degrades to `Set-Cookie` headers and says so in each finding's
`source` field.

Note: the analyzer's registered `Name()` can differ from its package name (e.g. package
`httpx` registers as `redirects`, package `robotscheck` registers as `robots`).

### Opt-in analyzer modes

`BuildRegistry(fetcher, opts)` takes a `runner.RegistryOptions`. Each field turns on checks
that are off by default, applied to the relevant analyzers via functional options; the zero
value leaves all of them off. When adding another opt-in mode, add a field here rather than
another positional bool.

- **`Specialized`** — deeper, more aggressive checks: `wordpress` security probes, the Shopify
  `/products.json` probe, `aeo` answer-lead, `geo` quotable-density. Surfaced as `--specialized`
  / `specialized` in MCP.
- **`SecurityAudit`** — the `security` analyzer's audit pass: TLS and certificate inspection,
  `Set-Cookie` attribute hygiene, and response-header policy. Unlike the WordPress probes it
  is passive (no extra fetches); it reads `crawler.Page.TLS`, captured by `HTTPFetcher` from
  the handshake and nil in headless render mode. Its findings are host-wide server
  configuration, so the audit aggregates them per host instead of emitting per page.
  Surfaced as `--security-audit` / `security_audit` in MCP.

## Adding an analyzer

1. Create a package under `internal/analyze/<yourcheck>/`.
2. Implement the `Analyzer` interface (use `analyze.EachPage` for per-page checks).
3. Register it in `runner.BuildRegistry`.
4. Add a unit test with an HTML fixture under `testdata/`.

No engine changes needed. This is exactly how the SEA and AI-search analyzers were added.

## Build, test, lint

```sh
make build    # go build -o gocrawl ./cmd/gocrawl
make test     # go test ./...
make vet      # go vet ./...
make lint     # golangci-lint run
make run      # build + crawl https://example.com --depth 1
```

CI (`.github/workflows/ci.yml`) runs `go mod verify`, `go vet`, `go build`,
`go test -race`, and golangci-lint on every push/PR to `main`. Run `gofmt` and `go vet`
before opening a PR. golangci-lint enables `errcheck`, `govet`, `ineffassign`, `staticcheck`,
`unused`, `misspell` (errcheck is relaxed in `_test.go` files).

## Conventions

- Keep analyzers focused and side-effect free; emit `analyze.Issue` values rather than
  printing.
- Add tests for new behavior, with HTML fixtures under the analyzer's `testdata/`.
- Crawl scope (depth, page cap, include/exclude regex, subdomains, rate limiting, robots
  compliance, follow-external, follow-nofollow, max duration) is all configurable; the engine
  bounds the walk with a fixed-size worker pool pulling from a FIFO frontier queue (bounding
  goroutines by `Concurrency` regardless of frontier size, and giving a true breadth-first
  traversal) and a `golang.org/x/time/rate` limiter.
- Headless rendering (`--render headless`) uses chromedp and needs a Chromium browser
  installed; raw-HTML crawling is the default and needs nothing extra.

## Documentation

Full reference docs live in `docs/` (start at `docs/README.md`): configuration, analyzers
(with every issue code), output formats, MCP server, architecture, and roadmap. `README.md`
is the user-facing entry point. Update the relevant doc when you change behavior or add an
analyzer.
