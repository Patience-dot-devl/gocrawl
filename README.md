# gocrawl

A highly-customizable, free and open-source (FOSS) website crawler for **SEO** and
**SEA** audits, written in Go.

`gocrawl` walks a website concurrently and runs a pipeline of pluggable **analyzers**
over every page — checking technical SEO, redirects, broken links, `robots.txt`,
`sitemap.xml` coverage, structured data, and more — then writes a JSON, CSV, or HTML report.

> **Status:** early, working vertical slice. Raw-HTML crawling, the core SEO analyzers, the
> SEA analyzers (UTM auditing, tracking-pixel detection, landing-page relevance), the
> AI-search analyzers (Answer Engine and Generative Engine Optimization), and headless
> rendering with lab-mode Core Web Vitals (LCP, FCP, CLS, TBT, TTFB) are all implemented. The
> design's whole point is that checks slot in as new analyzers without touching the engine —
> see the [Roadmap](#roadmap) for what's next.

## Why gocrawl

- **Customizable by design.** Every check is an independent analyzer you can enable,
  disable, or configure. Crawl scope (depth, page cap, include/exclude patterns,
  subdomains, rate limiting, robots compliance) is fully configurable.
- **Fast & portable.** Concurrent Go engine, single static binary, no runtime deps.
- **SEO, SEA, and AI search.** Ships with SEO/technical analyzers, SEA analyzers (UTM
  auditing, tracking-pixel detection, landing-page relevance), and AI-search analyzers
  (Answer Engine and Generative Engine Optimization) — each an independent check on the same
  interface.
- **Reports you can pipe or share.** JSON for tooling, CSV for spreadsheets, and a
  self-contained HTML page (inline CSS, no JS) for handing to stakeholders.

## Install

Works on **Windows, macOS, and Linux**. With [Go 1.26+](https://go.dev/dl/):

```sh
go install github.com/Patience-dot-devl/gocrawl/cmd/gocrawl@latest
```

Or build from source:

```sh
git clone https://github.com/Patience-dot-devl/gocrawl
cd gocrawl
make build      # produces ./gocrawl  (on Windows: go build -o gocrawl.exe ./cmd/gocrawl)
```

📖 **See [docs/install.md](docs/install.md)** for full per-platform instructions — PATH setup
on each OS, building without `make` on Windows, verifying the install, and the optional
Chromium browser needed for `--render headless`.

## Quick start

```sh
# Crawl one level deep and write a JSON report
gocrawl crawl https://example.com --depth 1 --out report.json

# CSV instead, with a page cap and higher concurrency
gocrawl crawl https://example.com --max-pages 200 --concurrency 8 \
  --format csv --out report.csv

# Self-contained HTML report to open in a browser
gocrawl crawl https://example.com --format html --out report.html

# Only run specific analyzers
gocrawl crawl https://example.com --analyzers seo,links,redirects

# List every available analyzer
gocrawl analyzers list

# Write a fully-commented example config you can edit
gocrawl init
gocrawl crawl https://example.com --config gocrawl.yaml
```

## Documentation

Full reference docs live in [`docs/`](docs/README.md):

- [Configuration](docs/configuration.md) — every option, flag, env var, and default.
- [Analyzers](docs/analyzers.md) — what each analyzer checks, with every issue code.
- [Output / report](docs/output.md) — the JSON, CSV, and HTML report formats.
- [MCP server](docs/mcp.md) — running as an MCP server and the tool schemas.
- [Architecture](docs/architecture.md) — how the engine and analyzer pipeline fit together.
- [Roadmap](docs/roadmap.md) — what's shipped, stubbed, and planned.

## Use as an MCP server (Conductor / Claude Code)

gocrawl can run as a [Model Context Protocol](https://modelcontextprotocol.io) server over
stdio, so agentic tools can drive crawls directly:

```sh
gocrawl mcp
```

It exposes two tools:

- **`crawl`** — run a crawl + analysis and return a structured JSON report. Arguments:
  `url` (required), `depth`, `max_pages`, `concurrency`, `render`, `analyzers`,
  `specialized`, `respect_robots`, `subdomains`, `include`, `exclude`.
- **`list_analyzers`** — list the available analyzers.

Register it with an MCP client. For example, Claude Code:

```sh
claude mcp add gocrawl -- gocrawl mcp
```

Or in a Conductor / Claude Code `mcp` config block:

```json
{
  "mcpServers": {
    "gocrawl": { "command": "gocrawl", "args": ["mcp"] }
  }
}
```

The agent can then call `crawl` with `{"url": "https://example.com", "depth": 2}` and reason
over the returned issues. See the full [MCP server guide](docs/mcp.md) for tool schemas and
examples.

## Configuration

Configuration is layered, in increasing precedence: **defaults → YAML config file →
environment variables (`GOCRAWL_*`) → command-line flags**. Generate a starting file with
`gocrawl init` (see [`configs/example.yaml`](configs/example.yaml)). The full option,
env-var, and default reference is in [docs/configuration.md](docs/configuration.md).

Key crawl options:

| Option | Flag | Description |
| --- | --- | --- |
| Max depth | `--depth` | How many link hops from the seed (0 = seed only) |
| Max pages | `--max-pages` | Hard cap on pages crawled |
| Concurrency | `--concurrency` | Parallel fetch workers |
| Rate limit | `--rate` | Max requests/second (0 = unlimited) |
| Adaptive delay | `--adaptive-delay` | Auto-slow the crawl on HTTP 429/503 (on by default; honors `Retry-After`) |
| Verbose | `--verbose` / `-v` | Log each fetch and rate-limit change to stderr |
| Rendering | `--render` | `raw` (default) or `headless` (chromedp — JS rendering + Core Web Vitals) |
| Scope | `--include` / `--exclude` | URL regex filters |
| Robots | `--respect-robots` | Obey `robots.txt` while crawling |
| Subdomains | `--subdomains` | Follow links to subdomains of the seed |
| Output | `--out` / `--format` | File path and `json` / `csv` / `html` |
| Site map | `--sitemap` / `--site-tree` | Also write a `sitemap.xml` and/or a clickable HTML site tree showing issues per page |
| Analyzers | `--analyzers` | Comma-separated allow-list |
| Specialized checks | `--specialized` | Enable opt-in checks: AI-search heuristics + WordPress security probes (off by default) |

## Analyzers (v1)

| Name | What it checks |
| --- | --- |
| `seo` | Title, meta description, meta robots, canonical, headings, `lang`, viewport, charset, OpenGraph/Twitter |
| `redirects` | Status codes, redirect chains/loops, slow responses, mixed content |
| `links` | Internal/external links, broken links, links to redirects, `nofollow` |
| `robots` | `robots.txt` discovery/parsing, declared sitemaps, disallow violations |
| `sitemap` | `sitemap.xml` discovery/parsing and crawl-coverage cross-check |
| `structured` | JSON-LD extraction and schema.org `@type` reporting |
| `perf` | Core Web Vitals (LCP, FCP, CLS, TBT, TTFB) against Google's thresholds — populated with `--render headless` |
| `utm` | **SEA** — UTM tagging on outbound links: partial/empty/duplicate params, casing |
| `tracking` | **SEA** — marketing/analytics tags (GTM, GA4, UA, Google Ads, Meta Pixel); missing/duplicate installs |
| `datalayer` | **SEA** — GTM/dataLayer audit: snippet wiring, Consent Mode, event inventory, GA4 e-commerce validation, duplicate conversions, PII; runtime checks need `--render headless` |
| `landing` | **SEA** — landing-page relevance: campaign-keyword alignment + indexability/HTTPS/title/H1 |
| `wordpress` | **CMS** — WordPress detection: version disclosure, plugin/emoji/jQuery-Migrate bloat, default tagline, ugly permalinks, conflicting SEO plugins, indexable attachment/search/archive pages, multilingual/WPML setup, leaked ACF markup, and opt-in xmlrpc/user-enumeration/directory-listing/readme probes |
| `aeo` | **AI search** — Answer Engine Optimization: FAQ/HowTo structured data, question headings, concise answers, direct-answer lead, snippet-friendly formatting |
| `geo` | **AI search** — Generative Engine Optimization: AI-crawler `robots.txt` policy, `/llms.txt` presence, author/date/main-content citability, JS-dependent content, quotable-data density |

The `aeo` direct-answer-lead and `geo` quotable-density checks, plus the `wordpress`
security-endpoint probes, are **opt-in** specialized checks, off by default; enable them with
`--specialized`. See
[docs/analyzers.md](docs/analyzers.md) for every issue code, severity, and threshold.

## How it works

```
seed URL ──▶ crawler engine ──▶ Result (pages, redirects, robots) ──▶ analyzer pipeline ──▶ report (json/csv/html)
              (concurrent,                                              (seo, links,
               scope + robots,                                          sitemap, …)
               redirect capture)
```

The engine never knows about specific checks; analyzers never fetch the crawl. The
`analyze.Analyzer` interface is the single seam between them — which is what makes new
checks cheap to add. See [CONTRIBUTING.md](CONTRIBUTING.md#adding-a-new-analyzer).

## Roadmap

Recently shipped: **headless rendering** via chromedp, **lab-mode Core Web Vitals**
(LCP, FCP, CLS, TBT, TTFB) in the `perf` analyzer, and the **HTML report** format. Next up:
resumable crawls and export integrations. See the full [feature roadmap](docs/roadmap.md)
for status on each.

## License

[MIT](LICENSE) © Patience-dot-devl
