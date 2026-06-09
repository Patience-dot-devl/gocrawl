# gocrawl

A highly-customizable, free and open-source (FOSS) website crawler for **SEO** and
**SEA** audits, written in Go.

`gocrawl` walks a website concurrently and runs a pipeline of pluggable **analyzers**
over every page — checking technical SEO, redirects, broken links, `robots.txt`,
`sitemap.xml` coverage, structured data, and more — then writes a JSON or CSV report.

> **Status:** early, working vertical slice. Raw-HTML crawling and the core analyzers
> are implemented. Headless rendering + Core Web Vitals are wired behind a flag but
> stubbed, and the SEA analyzers are on the roadmap (see [Roadmap](#roadmap)). The
> design's whole point is that these slot in as new analyzers without touching the engine.

## Why gocrawl

- **Customizable by design.** Every check is an independent analyzer you can enable,
  disable, or configure. Crawl scope (depth, page cap, include/exclude patterns,
  subdomains, rate limiting, robots compliance) is fully configurable.
- **Fast & portable.** Concurrent Go engine, single static binary, no runtime deps.
- **SEO and SEA.** Ships with SEO/technical analyzers; the analyzer interface is built so
  SEA checks (UTM auditing, tracking-pixel detection, landing-page relevance) drop in.
- **Reports you can pipe.** JSON for tooling, CSV for spreadsheets.

## Install

```sh
go install github.com/Patience-dot-devl/gocrawl/cmd/gocrawl@latest
```

Or build from source:

```sh
git clone https://github.com/Patience-dot-devl/gocrawl
cd gocrawl
make build      # produces ./gocrawl
```

## Quick start

```sh
# Crawl one level deep and write a JSON report
gocrawl crawl https://example.com --depth 1 --out report.json

# CSV instead, with a page cap and higher concurrency
gocrawl crawl https://example.com --max-pages 200 --concurrency 8 \
  --format csv --out report.csv

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
- [Output / report](docs/output.md) — the JSON and CSV report schema.
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
  `respect_robots`, `subdomains`, `include`, `exclude`.
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
| Rendering | `--render` | `raw` (default) or `headless` (stubbed) |
| Scope | `--include` / `--exclude` | URL regex filters |
| Robots | `--respect-robots` | Obey `robots.txt` while crawling |
| Subdomains | `--subdomains` | Follow links to subdomains of the seed |
| Output | `--out` / `--format` | File path and `json`/`csv` |
| Analyzers | `--analyzers` | Comma-separated allow-list |

## Analyzers (v1)

| Name | What it checks |
| --- | --- |
| `seo` | Title, meta description, meta robots, canonical, headings, `lang`, viewport, charset, OpenGraph/Twitter |
| `redirects` | Status codes, redirect chains/loops, slow responses, mixed content |
| `links` | Internal/external links, broken links, links to redirects, `nofollow` |
| `robots` | `robots.txt` discovery/parsing, declared sitemaps, disallow violations |
| `sitemap` | `sitemap.xml` discovery/parsing and crawl-coverage cross-check |
| `structured` | JSON-LD extraction and schema.org `@type` reporting |
| `perf` | Core Web Vitals — **stubbed**; requires headless rendering (roadmap) |

See [docs/analyzers.md](docs/analyzers.md) for every issue code, severity, and threshold.

## How it works

```
seed URL ──▶ crawler engine ──▶ Result (pages, redirects, robots) ──▶ analyzer pipeline ──▶ report (json/csv)
              (concurrent,                                              (seo, links,
               scope + robots,                                          sitemap, …)
               redirect capture)
```

The engine never knows about specific checks; analyzers never fetch the crawl. The
`analyze.Analyzer` interface is the single seam between them — which is what makes new
checks cheap to add. See [CONTRIBUTING.md](CONTRIBUTING.md#adding-a-new-analyzer).

## Roadmap

Next up: headless rendering (chromedp) + real Core Web Vitals, the **SEA analyzers**
(UTM auditing, tracking-pixel / GTM / GA4 / Meta Pixel detection, landing-page relevance),
and platform features like HTML reports and resumable crawls. See the full
[feature roadmap](docs/roadmap.md) for status on each.

## License

[MIT](LICENSE) © Patience-dot-devl
