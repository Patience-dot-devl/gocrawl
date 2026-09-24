# gocrawl

A highly-customizable, free and open-source (FOSS) website crawler for **SEO** and
**SEA** audits, written in Go.

`gocrawl` walks a website concurrently and runs a pipeline of pluggable **analyzers**
over every page — checking technical SEO, redirects, broken links, `robots.txt`,
`sitemap.xml` coverage, structured data, and more — then writes a JSON, CSV, or HTML report.

> **Status:** v0.6.0, actively developed. Twenty-five analyzers cover technical SEO, SEA,
> AI-search, and CMS-specific checks; raw-HTML and headless (chromedp) rendering with
> lab-mode Core Web Vitals; JSON/CSV/HTML reports; crawl storage & comparison over time; an
> MCP server; and a web app (`gocrawl serve`) with a REST API and embedded browser UI. The
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

To build with the browser UI embedded (needs [Node](https://nodejs.org/) too):

```sh
make web-build  # compiles web/ into internal/webserver/webui/dist/ (needs npm)
make build      # embeds that build into ./gocrawl
```

Plain `make build` without `make web-build` first still works — it embeds a placeholder page
for `gocrawl serve` instead of the real UI. Run `gocrawl path` after building to add the
binary's directory to your shell PATH.

📖 **See [docs/install.md](docs/install.md)** for full per-platform instructions — PATH setup
on each OS, building without `make` on Windows, verifying the install, and the optional
Chromium browser needed for `--render headless`.

## Quick start

```sh
# No arguments on an interactive terminal launches a guided menu (pick URL, depth,
# analyzers, output format, etc.) instead of requiring flags up front
gocrawl

# Crawl one level deep and write a JSON report
gocrawl crawl https://example.com --depth 1 --out report.json

# CSV instead, with a page cap and higher concurrency
gocrawl crawl https://example.com --max-pages 200 --concurrency 8 \
  --format csv --out report.csv

# Self-contained HTML report to open in a browser (with a visual site-map tab)
gocrawl crawl https://example.com --format html --out report.html

# Re-render a saved JSON report into HTML — no recrawl
gocrawl crawl  https://example.com --format json --out report.json
gocrawl render report.json --out report.html

# Only run specific analyzers
gocrawl crawl https://example.com --analyzers seo,links,redirects

# Save a crawl, then track progress over time
gocrawl crawl https://example.com --save           # store this crawl
gocrawl history                                     # list saved crawls
# ...fix some issues, recrawl with --save, then:
gocrawl compare example.com/<earlier> latest        # what's new / resolved
gocrawl compare before.json after.json --fail-on-new  # CI gate on regressions

# List every available analyzer
gocrawl analyzers list

# Write a fully-commented example config you can edit
gocrawl init
gocrawl crawl https://example.com --config gocrawl.yaml

# Verify a HubSpot redirect-rule export against the live site
gocrawl check-redirects --input redirects.csv --domain example.com --output results.csv

# Run as a web app: REST API + embedded browser UI on 127.0.0.1:8080
gocrawl serve

# Run as an MCP server over stdio, for agentic tools
gocrawl mcp
```

## Commands

| Command | What it does |
| --- | --- |
| `gocrawl` (no args) | Interactive menu on a terminal — pick URL, scope, analyzers, output |
| `gocrawl crawl <url>` | Crawl and write a report |
| `gocrawl render <report>` | Re-render a saved JSON report as another format, no recrawl |
| `gocrawl analyzers list` | List every available analyzer |
| `gocrawl init` | Write a fully-commented example YAML config |
| `gocrawl history [host]` | List crawls saved with `--save` |
| `gocrawl compare <base> <current>` | Diff two crawls: new / resolved / persisting issues |
| `gocrawl check-redirects` | Verify a redirect-rule CSV export against a live site |
| `gocrawl mcp` | Run as an MCP server over stdio |
| `gocrawl serve` | Run as a web app: REST API + embedded browser UI |
| `gocrawl path` | Add the binary's directory to your shell PATH |

Every command accepts `-c/--config <file>`; run `gocrawl <command> --help` for its flags.

## Documentation

Full reference docs live in [`docs/`](docs/README.md):

- [Configuration](docs/configuration.md) — every option, flag, env var, and default.
- [Analyzers](docs/analyzers.md) — what each analyzer checks, with every issue code.
- [Output / report](docs/output.md) — the JSON, CSV, and HTML report formats.
- [MCP server](docs/mcp.md) — running as an MCP server and the tool schemas.
- [Web UI](docs/web.md) — running `gocrawl serve`, the browser UI, and the REST API.
- [Redirect-rule verification](docs/redirect-check.md) — checking a redirect-rule CSV export against a live site.
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
  `specialized`, `security_audit`, `respect_robots`, `subdomains`, `include`, `exclude`.
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

## Use as a web app

```sh
gocrawl serve
```

Starts an HTTP server on `:8080` (override with `--addr`) exposing a REST API to start,
poll, cancel, and export crawls, and serves the built-in browser UI at the same address — a
form to start a crawl, a live-polling report view, and crawl history. See the full
[Web UI guide](docs/web.md) for the API reference and how to build the frontend from source.

## Configuration

Configuration is layered, in increasing precedence: **defaults → YAML config file →
environment variables (`GOCRAWL_*`) → command-line flags**. Generate a starting file with
`gocrawl init` (see [`configs/example.yaml`](configs/example.yaml)). The full option,
env-var, and default reference is in [docs/configuration.md](docs/configuration.md).

Key crawl options:

| Option | Flag | Description |
| --- | --- | --- |
| Max depth | `--depth` | How many link hops from the seed (0 = unlimited; the crawl is bounded by `--max-pages`) |
| Max pages | `--max-pages` | Hard cap on pages crawled — the primary bound on crawl size |
| Concurrency | `--concurrency` | Parallel fetch workers |
| Rate limit | `--rate` | Max requests/second (0 = unlimited) |
| Max duration | `--max-duration` | Wall-clock budget for the whole crawl, e.g. `90m` (0 = unlimited); on expiry the crawl stops early and still writes a partial report |
| Adaptive delay | `--adaptive-delay` | Auto-slow the crawl on HTTP 429/503 (on by default; honors `Retry-After`) |
| Verbose | `--verbose` / `-v` | Log each fetch and rate-limit change to stderr |
| Rendering | `--render` | `raw` (default) or `headless` (chromedp — JS rendering + Core Web Vitals) |
| Scope | `--include` / `--exclude` | URL regex filters |
| Robots | `--respect-robots` | Obey `robots.txt` while crawling |
| Subdomains | `--subdomains` | Follow links to subdomains of the seed |
| Output | `--out` / `--format` | File path and `json` / `csv` / `html` |
| Site map | `--sitemap` | Write a `sitemap.xml`; the HTML report also has a Site map tab that draws the crawl as a visual node-link diagram with issues per page |
| Analyzers | `--analyzers` | Comma-separated allow-list |
| Specialized checks | `--specialized` | Enable opt-in checks: AI-search heuristics, WordPress security probes, Shopify `/products.json` probe (off by default) |
| Security audit | `--security-audit` | Enable the opt-in TLS/certificate, cookie, and response-header audit (off by default) |
| Ignore external tagging | `--ignore-external-tagging` | Suppress the `utm` analyzer's tagging-quality warnings for links leaving the domain (on by default) |

## Analyzers

Twenty-five analyzers, run in registration order:

| Name | What it checks |
| --- | --- |
| `seo` | Title, meta description, meta/`X-Robots-Tag` robots directives, meta-refresh, canonical, headings, `lang`, viewport, charset, OpenGraph |
| `redirects` | Status codes, redirect chains/loops, slow responses, mixed content |
| `links` | Internal/external links, broken links, links to redirects, empty anchors, inbound-link counts |
| `robots` | `robots.txt` discovery/parsing, declared sitemaps, disallow violations |
| `sitemap` | `sitemap.xml` discovery/parsing and crawl-coverage cross-check |
| `structured` | JSON-LD extraction via a shared schema.org graph, rich-result field tiering (required/recommended/merchant) per `@type`, duplicate/conflicting-type detection, and markup-integrity checks (`@id` resolution, relative URLs, price/date formatting, price-vs-page mismatches) |
| `perf` | Core Web Vitals (LCP, FCP, CLS, TBT, TTFB) against Google's thresholds — populated with `--render headless` |
| `images` | Alt text (missing, empty, duplicated, filename-derived), non-descriptive filenames, missing `width`/`height` dimensions |
| `urls` | URL hygiene: uppercase paths, underscores, non-ASCII characters, overly long URLs |
| `security` | Insecure forms and response-header hygiene (baseline); TLS/certificate/cookie audit with `--security-audit` |
| `pagination` | `rel=next/prev` sequence detection and broken pagination targets |
| `hreflang` | `hreflang` code validity, missing `x-default`/self-reference, missing reciprocal links |
| `amp` | AMP-page detection, missing canonical/runtime, broken `amphtml` links |
| `duplicates` | Exact-duplicate body content, titles, and meta descriptions across pages |
| `content` | Thin pages (< 100 words) and pages well below the crawl's average word count |
| `botwall` | **Crawl integrity** — detects CAPTCHA / bot-challenge walls (reCAPTCHA, hCaptcha, Turnstile, Cloudflare/DataDome/AWS WAF/PerimeterX/Imperva) served instead of real content, so a silently-blocked crawl isn't mistaken for a clean audit |
| `wordpress` | **CMS** — WordPress detection: version disclosure, plugin/emoji/jQuery-Migrate bloat, default tagline, ugly permalinks, conflicting SEO plugins, indexable attachment/search/archive pages, multilingual/WPML setup, leaked ACF markup, and opt-in xmlrpc/user-enumeration/directory-listing/readme probes |
| `shopify` | **CMS** — Shopify detection: per-template structured-data coverage, theme/app schema conflicts, flattened product variants, indexable utility/faceted URLs, collection-nested product paths without a canonical, and an opt-in `/products.json` exposure probe |
| `utm` | **SEA** — UTM tagging on outbound links: partial/empty/duplicate params, casing |
| `tracking` | **SEA** — marketing/analytics tags (GTM, GA4, UA, Google Ads, Meta Pixel); missing/duplicate installs |
| `datalayer` | **SEA** — GTM/dataLayer audit: snippet wiring, Consent Mode, event inventory, GA4 e-commerce validation, duplicate conversions, PII; runtime checks need `--render headless` |
| `landing` | **SEA** — landing-page relevance: campaign-keyword alignment + indexability/HTTPS/title/H1 |
| `consent` | **SEA / compliance** — CMP detection, Google Consent Mode v2 configuration, and the tracking cookies and beacons served *before* consent (the crawl never clicks a banner, so every visit is a pre-consent one); use `--render headless` for the full cookie jar |
| `aeo` | **AI search** — Answer Engine Optimization: FAQ/HowTo structured data, question headings, concise answers, direct-answer lead, snippet-friendly formatting |
| `geo` | **AI search** — Generative Engine Optimization: AI-crawler `robots.txt` policy, `/llms.txt` presence, author/date/main-content citability, JS-dependent content, quotable-data density |

`seaurl` is a shared UTM-parsing helper used by `utm`/`tracking`, not a registered analyzer.

The `aeo` direct-answer-lead and `geo` quotable-density checks, the `wordpress`
security-endpoint probes, and the `shopify` `/products.json` exposure probe are **opt-in**
specialized checks, off by default; enable them with `--specialized`. See
[docs/analyzers.md](docs/analyzers.md) for every issue code, severity, and threshold.

### Security audit (opt-in)

`--security-audit` extends the `security` analyzer with a transport- and cookie-level pass:

- **TLS** — negotiated protocol version and cipher suite (obsolete versions, insecure suites).
- **Certificates** — expiry (30-day warning, 14-day error), validity window, self-signed
  certificates, chains missing their intermediates, SHA-1/MD5 signatures, undersized keys.
- **Cookies** — `Secure`, `HttpOnly` on session cookies, `SameSite`, the `__Host-`/`__Secure-`
  name prefixes, and lifetimes past the 400-day browser cap.
- **Response headers** — HSTS quality (`max-age`, `includeSubDomains`), framing protection,
  `Referrer-Policy`, and software-version disclosure.

It is **passive**: it reads the crawl's own responses and opens no extra connections, sends no
probes, and changes nothing about how the site is crawled. Because these findings describe
server configuration rather than individual pages, they are reported once per host. TLS and
certificate checks need `--render raw` (the default) — headless rendering does not expose the
handshake.

```sh
gocrawl crawl https://example.com --security-audit
```

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

Recently shipped: **`gocrawl serve`** (REST API + embedded web UI), Screaming-Frog-parity
analyzers (`images`, `urls`, `security`, `pagination`, `hreflang`, `amp`, `duplicates`,
`content`), **headless rendering** via chromedp with **lab-mode Core Web Vitals** in the
`perf` analyzer, and **crawl storage & comparison** (`--save` / `gocrawl history` /
`gocrawl compare`). Planned next: Internal Link Score, orphan-page detection, resumable
crawls, and API integrations (Search Console, PageSpeed Insights, backlink data). See the
full [feature roadmap](docs/roadmap.md) for status on each.

## License

[MIT](LICENSE) © Patience-dot-devl
