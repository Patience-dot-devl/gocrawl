# Feature roadmap

Where gocrawl is today and where it's headed. This is a direction document, not a dated
release plan — the project is an early, working vertical slice, so items are grouped by status
and theme rather than committed to timelines.

**Status legend:** ✅ shipped &nbsp;·&nbsp; 🚧 in progress / stubbed &nbsp;·&nbsp; 📋 planned

---

## ✅ Shipped (v1)

The current baseline. Everything here works today.

- **Concurrent crawl engine** — raw-HTML fetching with configurable depth, page cap,
  worker concurrency, and per-request timeout. ([`internal/crawler`](../internal/crawler))
- **Scope & politeness controls** — `robots.txt` compliance, rate limiting,
  include/exclude URL regexes, subdomain and external-link toggles, custom User-Agent.
- **Redirect capture** — full redirect chains recorded per page, with loop detection.
- **Ten analyzers** — the SEO/technical set `seo`, `redirects`, `links`, `robots`,
  `sitemap`, `structured`, `perf`, plus the SEA set `utm`, `tracking`, `landing`. See the
  [Analyzer reference](analyzers.md).
- **SEA analyzers** — UTM-parameter auditing (`utm`), tracking-pixel / GTM / GA4 / Meta-Pixel
  detection (`tracking`), and landing-page relevance (`landing`). Each was added as a new
  `analyze.Analyzer` with **no changes to the crawl engine** — the worked example for the
  analyzer architecture (see [Architecture](architecture.md#adding-an-analyzer)).
- **JSON & CSV reports** — with severity/analyzer/status summaries. See the
  [Output reference](output.md).
- **MCP server** — drive crawls from agentic tools via the `crawl` and `list_analyzers`
  tools. See the [MCP guide](mcp.md).
- **Layered configuration** — defaults → YAML → env → flags, with `gocrawl init` to
  scaffold a config. See the [Configuration reference](configuration.md).

## 🚧 In progress / stubbed

Wired into the codebase but not yet active. The seams already exist.

- **Headless rendering (chromedp)** — `render: headless` / `--render headless` is accepted
  and routes through [`internal/render`](../internal/render), but currently falls back to raw
  fetching and annotates pages that rendering isn't active yet.
- **Real Core Web Vitals** — the [`perf`](analyzers.md#perf--performance--core-web-vitals-stub)
  analyzer is a stub. It reports a response-time (TTFB) proxy and a notice; LCP, CLS, INP, FCP,
  and full TTFB require headless rendering to land first.

## 📋 Planned — platform

Broader capabilities beyond individual checks.

- **HTML report output** — a browsable report alongside JSON and CSV.
- **Resumable crawls** — checkpoint and continue large crawls.
- **Export integrations** — push results to external sinks/tools.

---

> Want to help, or have a check in mind? The analyzer interface is the extension point — see
> [CONTRIBUTING.md](../CONTRIBUTING.md). The README carries a short summary of this roadmap.
